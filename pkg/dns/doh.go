package dns

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

const (
	DnsMessageContentType = "application/dns-message"
	DnsQueryTimeout       = 5 * time.Second
)

type DnsProviderInfo struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	DohURL      string `json:"doh_url,omitempty"`
	Description string `json:"description"`
}

var StandardDnsProviders = []DnsProviderInfo{
	{
		ID:          "SYSTEM_DEFAULT",
		Label:       "Default (Edge Resolved)",
		DohURL:      "",
		Description: "Resolved directly by Fastly edge proxy node",
	},
	{
		ID:          "CLOUDFLARE",
		Label:       "Cloudflare",
		DohURL:      "https://1.1.1.1/dns-query",
		Description: "Fast, privacy-focused DoH (1.1.1.1)",
	},
	{
		ID:          "GOOGLE",
		Label:       "Google",
		DohURL:      "https://8.8.8.8/dns-query",
		Description: "Google Public DNS DoH (8.8.8.8)",
	},
	{
		ID:          "QUAD9",
		Label:       "Quad9",
		DohURL:      "https://9.9.9.9/dns-query",
		Description: "Malware-blocking security DoH (9.9.9.9)",
	},
	{
		ID:          "ADGUARD",
		Label:       "AdGuard",
		DohURL:      "https://dns.adguard-dns.com/dns-query",
		Description: "Ad and tracker blocking DoH",
	},
	{
		ID:          "CUSTOM",
		Label:       "Custom DoH",
		DohURL:      "",
		Description: "User-specified RFC 8484 DoH endpoint",
	},
}

type DohClient struct {
	mu         sync.RWMutex
	httpClient *http.Client
	socksPort  int
}

func NewDohClient(socksPort int) *DohClient {
	c := &DohClient{socksPort: socksPort}
	c.rebuildClient()
	return c
}

func (c *DohClient) SetSocksPort(socksPort int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.socksPort = socksPort
	c.rebuildClient()
}

func (c *DohClient) rebuildClient() {
	var transport *http.Transport
	if c.socksPort > 0 {
		proxyAddr := fmt.Sprintf("127.0.0.1:%d", c.socksPort)
		dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
		if err == nil {
			transport = &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialer.Dial(network, addr)
				},
				DisableKeepAlives: false,
				MaxIdleConns:      4,
				IdleConnTimeout:   60 * time.Second,
			}
		}
	}

	if transport == nil {
		transport = &http.Transport{
			DisableKeepAlives: false,
			MaxIdleConns:      4,
			IdleConnTimeout:   60 * time.Second,
		}
	}

	c.httpClient = &http.Client{
		Transport: transport,
		Timeout:   DnsQueryTimeout,
	}
}

// QueryDoh sends an RFC 8484 DNS message query to the specified DoH URL.
func (c *DohClient) QueryDoh(ctx context.Context, endpointURL string, queryMsg []byte) ([]byte, error) {
	if strings.TrimSpace(endpointURL) == "" {
		endpointURL = "https://1.1.1.1/dns-query"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(queryMsg))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", DnsMessageContentType)
	req.Header.Set("Content-Type", DnsMessageContentType)

	c.mu.RLock()
	client := c.httpClient
	c.mu.RUnlock()

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DoH query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH server returned status %d", resp.StatusCode)
	}

	answer, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err != nil {
		return nil, err
	}
	if len(answer) < 12 {
		return nil, errors.New("DoH response too short to be a valid DNS message")
	}
	return answer, nil
}
