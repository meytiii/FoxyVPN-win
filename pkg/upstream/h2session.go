package upstream

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

const (
	ClientPreface          = "PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n"
	MaxH2FrameSize         = 16384
	DefaultConnectTimeout  = 15 * time.Second
	KeepalivePingInterval  = 25 * time.Second
	MozillaVpnUserAgentH2  = "MozillaVPN/2.35.0 (sys:windows; iap:true)"
)

// H2Session manages a multiplexed HTTP/2 TLS connection to a Fastly edge proxy.
type H2Session struct {
	remoteHost string
	remotePort int
	splitMode  TlsSplitMode
	rawConn    net.Conn
	tlsConn    *tls.Conn
	framer     *http2.Framer
	writeMu    sync.Mutex
	encoder    *hpack.Encoder
	headerBuf  bytes.Buffer

	streamsMu    sync.RWMutex
	streams      map[uint32]*tunneledConn
	nextStreamID uint32

	closed   atomic.Bool
	closeErr error
	ctx      context.Context
	cancel   context.CancelFunc
}

// DialH2Session establishes an HTTP/2 TLS tunnel connection to a proxy node.
func DialH2Session(host string, port int, splitMode TlsSplitMode) (*H2Session, error) {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{
		Timeout: 15 * time.Second,
	}

	tcpConn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial TCP %s: %w", addr, err)
	}

	var baseConn net.Conn = tcpConn
	if splitMode != SplitOff {
		baseConn = NewSplitConn(tcpConn, splitMode)
	}

	tlsConfig := &tls.Config{
		ServerName:         host,
		NextProtos:         []string{"h2"},
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: false,
	}

	tlsConn := tls.Client(baseConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		_ = tcpConn.Close()
		return nil, fmt.Errorf("TLS handshake failed for %s: %w", host, err)
	}

	negotiatedProto := tlsConn.ConnectionState().NegotiatedProtocol
	if negotiatedProto != "h2" {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("server negotiated %q instead of HTTP/2 (h2)", negotiatedProto)
	}

	// Send HTTP/2 connection preface
	if _, err := tlsConn.Write([]byte(ClientPreface)); err != nil {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("failed sending HTTP/2 preface: %w", err)
	}

	framer := http2.NewFramer(tlsConn, tlsConn)
	framer.MaxHeaderListSize = 1048576

	// Send initial SETTINGS frame
	if err := framer.WriteSettings(
		http2.Setting{ID: http2.SettingEnablePush, Val: 0},
		http2.Setting{ID: http2.SettingInitialWindowSize, Val: 1048576},
		http2.Setting{ID: http2.SettingMaxConcurrentStreams, Val: 250},
	); err != nil {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("failed sending SETTINGS frame: %w", err)
	}

	// Send initial connection-level WINDOW_UPDATE
	_ = framer.WriteWindowUpdate(0, 1048576)

	ctx, cancel := context.WithCancel(context.Background())
	sess := &H2Session{
		remoteHost:   host,
		remotePort:   port,
		splitMode:    splitMode,
		rawConn:      tcpConn,
		tlsConn:      tlsConn,
		framer:       framer,
		streams:      make(map[uint32]*tunneledConn),
		nextStreamID: 1,
		ctx:          ctx,
		cancel:       cancel,
	}
	sess.encoder = hpack.NewEncoder(&sess.headerBuf)

	go sess.readLoop()
	go sess.pingLoop()

	return sess, nil
}

// OpenTunnel initiates an HTTP/2 CONNECT tunnel to targetHost:targetPort.
func (s *H2Session) OpenTunnel(targetHost string, targetPort int, bearerToken string) (net.Conn, error) {
	if s.closed.Load() {
		return nil, errors.New("upstream HTTP/2 session is closed")
	}

	streamID := atomic.AddUint32(&s.nextStreamID, 2)
	pipeReader, pipeWriter := io.Pipe()

	tunnel := &tunneledConn{
		session:      s,
		streamID:     streamID,
		targetHost:   targetHost,
		targetPort:   targetPort,
		pipeReader:   pipeReader,
		pipeWriter:   pipeWriter,
		responseChan: make(chan int, 1),
	}

	s.streamsMu.Lock()
	s.streams[streamID] = tunnel
	s.streamsMu.Unlock()

	authority := net.JoinHostPort(targetHost, strconv.Itoa(targetPort))

	s.writeMu.Lock()
	s.headerBuf.Reset()
	_ = s.encoder.WriteField(hpack.HeaderField{Name: ":method", Value: "CONNECT"})
	_ = s.encoder.WriteField(hpack.HeaderField{Name: ":authority", Value: authority})
	_ = s.encoder.WriteField(hpack.HeaderField{Name: "proxy-authorization", Value: "Bearer " + bearerToken})
	_ = s.encoder.WriteField(hpack.HeaderField{Name: "user-agent", Value: MozillaVpnUserAgentH2})

	err := s.framer.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      streamID,
		BlockFragment: s.headerBuf.Bytes(),
		EndStream:     false,
		EndHeaders:    true,
	})
	s.writeMu.Unlock()

	if err != nil {
		s.removeStream(streamID)
		_ = tunnel.Close()
		return nil, fmt.Errorf("failed to write CONNECT headers: %w", err)
	}

	select {
	case status := <-tunnel.responseChan:
		if status < 200 || status >= 300 {
			s.removeStream(streamID)
			_ = tunnel.Close()
			return nil, fmt.Errorf("upstream rejected CONNECT to %s with HTTP %d", authority, status)
		}
		return tunnel, nil

	case <-time.After(DefaultConnectTimeout):
		s.removeStream(streamID)
		_ = tunnel.Close()
		return nil, fmt.Errorf("timed out waiting for CONNECT response to %s", authority)

	case <-s.ctx.Done():
		s.removeStream(streamID)
		_ = tunnel.Close()
		return nil, errors.New("session closed while awaiting tunnel handshake")
	}
}

func (s *H2Session) readLoop() {
	defer func() {
		s.Close()
	}()

	for {
		frame, err := s.framer.ReadFrame()
		if err != nil {
			s.closeErr = err
			return
		}

		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				s.writeMu.Lock()
				_ = s.framer.WriteSettingsAck()
				s.writeMu.Unlock()
			}

		case *http2.HeadersFrame:
			statusCode := 0
			decoder := hpack.NewDecoder(4096, func(hf hpack.HeaderField) {
				if hf.Name == ":status" {
					if code, err := strconv.Atoi(hf.Value); err == nil {
						statusCode = code
					}
				}
			})
			_, _ = decoder.Write(f.HeaderBlockFragment())

			s.streamsMu.RLock()
			tunnel, exists := s.streams[f.StreamID]
			s.streamsMu.RUnlock()

			if exists && tunnel != nil {
				select {
				case tunnel.responseChan <- statusCode:
				default:
				}
			}

		case *http2.DataFrame:
			streamID := f.StreamID
			data := f.Data()

			s.streamsMu.RLock()
			tunnel, exists := s.streams[streamID]
			s.streamsMu.RUnlock()

			if exists && tunnel != nil && len(data) > 0 {
				_, _ = tunnel.pipeWriter.Write(data)

				// Replenish stream and connection window frames
				s.writeMu.Lock()
				_ = s.framer.WriteWindowUpdate(0, uint32(len(data)))
				_ = s.framer.WriteWindowUpdate(streamID, uint32(len(data)))
				s.writeMu.Unlock()
			}

			if f.StreamEnded() {
				if exists && tunnel != nil {
					_ = tunnel.pipeWriter.Close()
				}
				s.removeStream(streamID)
			}

		case *http2.PingFrame:
			if !f.IsAck() {
				s.writeMu.Lock()
				_ = s.framer.WritePing(true, f.Data)
				s.writeMu.Unlock()
			}

		case *http2.RSTStreamFrame:
			s.streamsMu.RLock()
			tunnel, exists := s.streams[f.StreamID]
			s.streamsMu.RUnlock()
			if exists && tunnel != nil {
				_ = tunnel.pipeWriter.CloseWithError(fmt.Errorf("stream reset by peer: code %v", f.ErrCode))
			}
			s.removeStream(f.StreamID)

		case *http2.GoAwayFrame:
			s.closeErr = fmt.Errorf("server sent GOAWAY: %v", f.ErrCode)
			return
		}
	}
}

func (s *H2Session) pingLoop() {
	ticker := time.NewTicker(KeepalivePingInterval)
	defer ticker.Stop()

	var pingData [8]byte
	for {
		select {
		case <-ticker.C:
			if s.closed.Load() {
				return
			}
			s.writeMu.Lock()
			_ = s.framer.WritePing(false, pingData)
			s.writeMu.Unlock()
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *H2Session) removeStream(streamID uint32) {
	s.streamsMu.Lock()
	delete(s.streams, streamID)
	s.streamsMu.Unlock()
}

// IsAlive returns true if the session is open and active.
func (s *H2Session) IsAlive() bool {
	return !s.closed.Load()
}

// Close terminates the session and all constituent tunneled connections.
func (s *H2Session) Close() error {
	if s.closed.Swap(true) {
		return nil
	}

	s.cancel()

	s.streamsMu.Lock()
	for id, tunnel := range s.streams {
		_ = tunnel.pipeWriter.CloseWithError(errors.New("parent session closed"))
		delete(s.streams, id)
	}
	s.streamsMu.Unlock()

	_ = s.tlsConn.Close()
	_ = s.rawConn.Close()
	return nil
}

// tunneledConn implements net.Conn over an HTTP/2 CONNECT data stream.
type tunneledConn struct {
	session      *H2Session
	streamID     uint32
	targetHost   string
	targetPort   int
	pipeReader   *io.PipeReader
	pipeWriter   *io.PipeWriter
	responseChan chan int
	closed       atomic.Bool
}

func (c *tunneledConn) Read(b []byte) (int, error) {
	return c.pipeReader.Read(b)
}

func (c *tunneledConn) Write(b []byte) (int, error) {
	if c.closed.Load() || c.session.closed.Load() {
		return 0, io.ErrClosedPipe
	}

	total := len(b)
	for len(b) > 0 {
		chunkSize := len(b)
		if chunkSize > MaxH2FrameSize {
			chunkSize = MaxH2FrameSize
		}

		c.session.writeMu.Lock()
		err := c.session.framer.WriteData(c.streamID, false, b[:chunkSize])
		c.session.writeMu.Unlock()

		if err != nil {
			return total - len(b), err
		}
		b = b[chunkSize:]
	}

	return total, nil
}

func (c *tunneledConn) Close() error {
	if c.closed.Swap(true) {
		return nil
	}

	c.session.writeMu.Lock()
	_ = c.session.framer.WriteRSTStream(c.streamID, http2.ErrCodeNo)
	c.session.writeMu.Unlock()

	_ = c.pipeWriter.Close()
	_ = c.pipeReader.Close()
	c.session.removeStream(c.streamID)
	return nil
}

func (c *tunneledConn) LocalAddr() net.Addr {
	return c.session.rawConn.LocalAddr()
}

func (c *tunneledConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: c.targetPort,
	}
}

func (c *tunneledConn) SetDeadline(t time.Time) error      { return nil }
func (c *tunneledConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *tunneledConn) SetWriteDeadline(t time.Time) error { return nil }
