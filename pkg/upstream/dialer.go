package upstream

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

type UpstreamDialConfig struct {
	Host                 string
	Port                 int
	CustomEdgeAddress    string
	SplitMode            TlsSplitMode
	UpstreamProxyEnabled bool
	UpstreamProxyType    string // "SOCKS5" or "HTTP"
	UpstreamProxyHost    string
	UpstreamProxyPort    int
	UpstreamProxyUser    string
	UpstreamProxyPass    string
}

// DialBaseConnection establishes the underlying TCP transport, routing through an upstream proxy if configured.
func DialBaseConnection(ctx context.Context, cfg UpstreamDialConfig) (net.Conn, error) {
	targetHost := cfg.Host
	targetPort := cfg.Port

	// If custom edge host/IP is provided, connect to it instead
	if strings.TrimSpace(cfg.CustomEdgeAddress) != "" {
		customEdge := strings.TrimSpace(cfg.CustomEdgeAddress)
		if h, p, err := net.SplitHostPort(customEdge); err == nil {
			targetHost = h
			if parsedPort, err := strconv.Atoi(p); err == nil && parsedPort > 0 {
				targetPort = parsedPort
			}
		} else {
			targetHost = customEdge
		}
	}

	targetAddr := net.JoinHostPort(targetHost, strconv.Itoa(targetPort))

	if !cfg.UpstreamProxyEnabled || strings.TrimSpace(cfg.UpstreamProxyHost) == "" {
		dialer := &net.Dialer{
			Timeout: 15 * time.Second,
		}
		return dialer.DialContext(ctx, "tcp", targetAddr)
	}

	proxyPort := cfg.UpstreamProxyPort
	if proxyPort <= 0 {
		proxyPort = 1080
	}
	proxyAddr := net.JoinHostPort(cfg.UpstreamProxyHost, strconv.Itoa(proxyPort))

	if strings.EqualFold(cfg.UpstreamProxyType, "HTTP") {
		// HTTP CONNECT proxy dialer
		dialer := &net.Dialer{Timeout: 15 * time.Second}
		pConn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to dial HTTP upstream proxy %s: %w", proxyAddr, err)
		}

		req, err := http.NewRequest(http.MethodConnect, "//"+targetAddr, nil)
		if err != nil {
			_ = pConn.Close()
			return nil, err
		}
		req.Host = targetAddr
		if cfg.UpstreamProxyUser != "" {
			auth := base64.StdEncoding.EncodeToString([]byte(cfg.UpstreamProxyUser + ":" + cfg.UpstreamProxyPass))
			req.Header.Set("Proxy-Authorization", "Basic "+auth)
		}

		if err := req.Write(pConn); err != nil {
			_ = pConn.Close()
			return nil, fmt.Errorf("failed writing CONNECT to upstream proxy: %w", err)
		}

		resp, err := http.ReadResponse(bufio.NewReader(pConn), req)
		if err != nil {
			_ = pConn.Close()
			return nil, fmt.Errorf("failed reading response from upstream proxy: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = pConn.Close()
			return nil, fmt.Errorf("upstream HTTP proxy returned status %d %s", resp.StatusCode, resp.Status)
		}
		return pConn, nil
	}

	// SOCKS5 proxy dialer
	var auth *proxy.Auth
	if cfg.UpstreamProxyUser != "" {
		auth = &proxy.Auth{
			User:     cfg.UpstreamProxyUser,
			Password: cfg.UpstreamProxyPass,
		}
	}

	dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed creating upstream SOCKS5 dialer: %w", err)
	}

	if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
		return contextDialer.DialContext(ctx, "tcp", targetAddr)
	}
	return dialer.Dial("tcp", targetAddr)
}

// IsValidProxyURL checks if a string parses as a valid proxy URL.
func IsValidProxyURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Host != ""
}
