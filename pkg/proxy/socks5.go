package proxy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
)

type DialFunc func(address string) (net.Conn, error)

type TrafficStats struct {
	BytesUploaded   atomic.Int64
	BytesDownloaded atomic.Int64
}

// Socks5Server implements an RFC 1928 SOCKS5 proxy server.
type Socks5Server struct {
	listener net.Listener
	dialFunc DialFunc
	stats    *TrafficStats
	mu       sync.Mutex
	running  bool
}

// NewSocks5Server creates a SOCKS5 proxy instance.
func NewSocks5Server(dialFunc DialFunc, stats *TrafficStats) *Socks5Server {
	return &Socks5Server{
		dialFunc: dialFunc,
		stats:    stats,
	}
}

// Start begins listening for SOCKS5 connections on the specified local address.
func (s *Socks5Server) Start(addr string) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to bind SOCKS5 server on %s: %w", addr, err)
	}

	s.listener = listener
	s.running = true
	s.mu.Unlock()

	go s.acceptLoop()
	return nil
}

func (s *Socks5Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			running := s.running
			s.mu.Unlock()
			if !running {
				return
			}
			continue
		}
		go s.handleClient(conn)
	}
}

func (s *Socks5Server) handleClient(clientConn net.Conn) {
	defer clientConn.Close()

	destAddr, err := s.performHandshake(clientConn)
	if err != nil {
		return
	}

	upstreamConn, err := s.dialFunc(destAddr)
	if err != nil {
		// Send general SOCKS server failure (0x01)
		_, _ = clientConn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer upstreamConn.Close()

	// Send success reply (0x00)
	if _, err := clientConn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}

	relayTraffic(clientConn, upstreamConn, s.stats)
}

func (s *Socks5Server) performHandshake(conn net.Conn) (string, error) {
	// Negotiation phase
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", err
	}

	if header[0] != 0x05 {
		return "", fmt.Errorf("unsupported socks version: %d", header[0])
	}

	numMethods := int(header[1])
	methods := make([]byte, numMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return "", err
	}

	// NO AUTHENTICATION REQUIRED (0x00)
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return "", err
	}

	// Request phase
	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, reqHeader); err != nil {
		return "", err
	}

	if reqHeader[0] != 0x05 {
		return "", fmt.Errorf("invalid socks request version: %d", reqHeader[0])
	}

	cmd := reqHeader[1]
	if cmd != 0x01 { // CONNECT
		_, _ = conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // Command not supported
		return "", fmt.Errorf("unsupported command: %d", cmd)
	}

	atyp := reqHeader[3]
	var host string

	switch atyp {
	case 0x01: // IPv4
		ipv4 := make([]byte, 4)
		if _, err := io.ReadFull(conn, ipv4); err != nil {
			return "", err
		}
		host = net.IP(ipv4).String()

	case 0x03: // Domain name
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", err
		}
		domainLen := int(lenBuf[0])
		domainBuf := make([]byte, domainLen)
		if _, err := io.ReadFull(conn, domainBuf); err != nil {
			return "", err
		}
		host = string(domainBuf)

	case 0x04: // IPv6
		ipv6 := make([]byte, 16)
		if _, err := io.ReadFull(conn, ipv6); err != nil {
			return "", err
		}
		host = net.IP(ipv6).String()

	default:
		_, _ = conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // Address type not supported
		return "", errors.New("unsupported address type")
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", err
	}
	port := binary.BigEndian.Uint16(portBuf)

	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

// Stop shuts down the SOCKS5 proxy listener.
func (s *Socks5Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// CountingReader tracks read bytes into atomic counter.
type CountingReader struct {
	reader  io.Reader
	counter *atomic.Int64
}

func (c *CountingReader) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	if n > 0 && c.counter != nil {
		c.counter.Add(int64(n))
	}
	return n, err
}

func relayTraffic(local, remote net.Conn, stats *TrafficStats) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Local to Remote (Upload)
	go func() {
		defer wg.Done()
		var r io.Reader = local
		if stats != nil {
			r = &CountingReader{reader: local, counter: &stats.BytesUploaded}
		}
		_, _ = io.Copy(remote, r)
		if tcpConn, ok := remote.(*net.TCPConn); ok {
			_ = tcpConn.CloseWrite()
		}
	}()

	// Remote to Local (Download)
	go func() {
		defer wg.Done()
		var r io.Reader = remote
		if stats != nil {
			r = &CountingReader{reader: remote, counter: &stats.BytesDownloaded}
		}
		_, _ = io.Copy(local, r)
		if tcpConn, ok := local.(*net.TCPConn); ok {
			_ = tcpConn.CloseWrite()
		}
	}()

	wg.Wait()
}
