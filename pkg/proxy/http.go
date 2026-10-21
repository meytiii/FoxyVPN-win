package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// HttpProxyServer handles HTTP CONNECT and transparent HTTP requests.
type HttpProxyServer struct {
	listener net.Listener
	dialFunc DialFunc
	stats    *TrafficStats
	mu       sync.Mutex
	running  bool
}

// NewHttpProxyServer creates a local HTTP proxy server.
func NewHttpProxyServer(dialFunc DialFunc, stats *TrafficStats) *HttpProxyServer {
	return &HttpProxyServer{
		dialFunc: dialFunc,
		stats:    stats,
	}
}

// Start begins listening on the designated local address.
func (s *HttpProxyServer) Start(addr string) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to bind HTTP proxy server on %s: %w", addr, err)
	}

	s.listener = listener
	s.running = true
	s.mu.Unlock()

	go s.acceptLoop()
	return nil
}

func (s *HttpProxyServer) acceptLoop() {
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

func (s *HttpProxyServer) handleClient(clientConn net.Conn) {
	defer clientConn.Close()

	reader := bufio.NewReader(clientConn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}

	if req.Method == http.MethodConnect {
		s.handleConnect(clientConn, req)
		return
	}

	s.handlePlainHTTP(clientConn, req, reader)
}

func (s *HttpProxyServer) handleConnect(clientConn net.Conn, req *http.Request) {
	destAddr := req.RequestURI
	if !strings.Contains(destAddr, ":") {
		destAddr = net.JoinHostPort(destAddr, "443")
	}

	upstreamConn, err := s.dialFunc(destAddr)
	if err != nil {
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer upstreamConn.Close()

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	relayTraffic(clientConn, upstreamConn, s.stats)
}

func (s *HttpProxyServer) handlePlainHTTP(clientConn net.Conn, req *http.Request, reader *bufio.Reader) {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	if host == "" {
		_, _ = clientConn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}

	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, "80")
	}

	upstreamConn, err := s.dialFunc(host)
	if err != nil {
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer upstreamConn.Close()

	// Rewrite request to relative URI
	if req.URL.IsAbs() {
		req.URL = &url.URL{
			Path:     req.URL.Path,
			RawPath:  req.URL.RawPath,
			RawQuery: req.URL.RawQuery,
		}
	}
	req.RequestURI = ""

	if s.stats != nil {
		s.stats.BytesUploaded.Add(int64(req.ContentLength))
	}

	if err := req.Write(upstreamConn); err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// Stream response back to client
	go func() {
		defer wg.Done()
		var r io.Reader = upstreamConn
		if s.stats != nil {
			r = &CountingReader{reader: upstreamConn, counter: &s.stats.BytesDownloaded}
		}
		_, _ = io.Copy(clientConn, r)
	}()

	wg.Wait()
}

// Stop terminates the HTTP proxy listener.
func (s *HttpProxyServer) Stop() error {
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
