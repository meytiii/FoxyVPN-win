package upstream

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

// TokenProvider defines a function that provides an active Guardian proxy JWT token.
type TokenProvider func() (string, error)

// UpstreamManager manages the lifecycle, pooling, and health of upstream HTTP/2 sessions.
type UpstreamManager struct {
	mu            sync.RWMutex
	currentConfig UpstreamDialConfig
	currentHost   string
	currentPort   int
	splitMode     TlsSplitMode
	tokenProvider TokenProvider
	session       *H2Session
	lastPingMs    int64
}

// NewUpstreamManager creates an upstream session manager.
func NewUpstreamManager(tokenProvider TokenProvider) *UpstreamManager {
	return &UpstreamManager{
		tokenProvider: tokenProvider,
		splitMode:     SplitFirstByte,
	}
}

// SetSplitMode updates the TLS ClientHello split strategy.
func (m *UpstreamManager) SetSplitMode(mode TlsSplitMode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.splitMode = mode
	m.currentConfig.SplitMode = mode
}

// ConnectWithConfig establishes the multiplexed H2 tunnel using specified dial configuration.
func (m *UpstreamManager) ConnectWithConfig(cfg UpstreamDialConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session != nil {
		_ = m.session.Close()
		m.session = nil
	}

	start := time.Now()
	sess, err := DialH2SessionWithConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed connecting to upstream proxy %s:%d: %w", cfg.Host, cfg.Port, err)
	}

	m.currentConfig = cfg
	m.currentHost = cfg.Host
	m.currentPort = cfg.Port
	m.session = sess
	m.lastPingMs = time.Since(start).Milliseconds()

	return nil
}

// Connect selects an upstream edge server and establishes the multiplexed H2 tunnel.
func (m *UpstreamManager) Connect(host string, port int) error {
	m.mu.RLock()
	cfg := m.currentConfig
	split := m.splitMode
	m.mu.RUnlock()

	cfg.Host = host
	cfg.Port = port
	cfg.SplitMode = split

	return m.ConnectWithConfig(cfg)
}

// Dial connects to a remote target through the active upstream HTTP/2 CONNECT tunnel.
func (m *UpstreamManager) Dial(targetAddress string) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(targetAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid destination address %s: %w", targetAddress, err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port in %s: %w", targetAddress, err)
	}

	token, err := m.tokenProvider()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve valid Guardian token: %w", err)
	}

	m.mu.RLock()
	sess := m.session
	cfg := m.currentConfig
	m.mu.RUnlock()

	if sess == nil || !sess.IsAlive() {
		// Attempt reconnect
		if cfg.Host == "" {
			return nil, errors.New("no upstream proxy node configured")
		}
		if err := m.ConnectWithConfig(cfg); err != nil {
			return nil, fmt.Errorf("failed to reconnect upstream: %w", err)
		}
		m.mu.RLock()
		sess = m.session
		m.mu.RUnlock()
	}

	conn, err := sess.OpenTunnel(host, port, token)
	if err != nil {
		// If session died during dial, try one reconnect
		if !sess.IsAlive() && cfg.Host != "" {
			_ = m.ConnectWithConfig(cfg)
			m.mu.RLock()
			sess = m.session
			m.mu.RUnlock()
			if sess != nil && sess.IsAlive() {
				return sess.OpenTunnel(host, port, token)
			}
		}
		return nil, err
	}

	return conn, nil
}

// Ping measures RTT to current host.
func (m *UpstreamManager) Ping() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastPingMs
}

// Close disconnects the upstream session.
func (m *UpstreamManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session != nil {
		err := m.session.Close()
		m.session = nil
		return err
	}
	return nil
}

// CurrentNode returns the active edge proxy host and port.
func (m *UpstreamManager) CurrentNode() (string, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentHost, m.currentPort
}
