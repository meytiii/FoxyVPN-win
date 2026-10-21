package exitcheck

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const (
	CloudflareTraceURL = "https://www.cloudflare.com/cdn-cgi/trace"
	CheckTimeout       = 10 * time.Second
)

type IpLocationInfo struct {
	IP          string `json:"ip"`
	CountryCode string `json:"country_code"`
	IsProtected bool   `json:"is_protected"`
}

type ExitChecker struct {
	httpClient *http.Client
}

func NewExitChecker() *ExitChecker {
	return &ExitChecker{
		httpClient: &http.Client{
			Timeout: CheckTimeout,
		},
	}
}

// FetchDirectIpInfo retrieves public IP and country without proxy.
func (c *ExitChecker) FetchDirectIpInfo(ctx context.Context) (*IpLocationInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, CloudflareTraceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "MozillaVPN/2.35.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("direct IP check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("direct IP check returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	info, err := parseTrace(string(body))
	if err != nil {
		return nil, err
	}
	info.IsProtected = false
	return info, nil
}

// FetchProxiedIpInfo routes through local SOCKS5 proxy to verify egress IP and country.
func (c *ExitChecker) FetchProxiedIpInfo(ctx context.Context, socksPort int) (*IpLocationInfo, error) {
	if socksPort <= 0 {
		socksPort = 1080
	}
	proxyAddr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to configure SOCKS5 dialer: %w", err)
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   CheckTimeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, CloudflareTraceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "MozillaVPN/2.35.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxied exit check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxied exit check returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	info, err := parseTrace(string(body))
	if err != nil {
		return nil, err
	}
	info.IsProtected = true
	return info, nil
}

// VerifyExitCountry confirms that tunnel egress country matches expected country.
func (c *ExitChecker) VerifyExitCountry(ctx context.Context, socksPort int, expectedCountry string) (string, error) {
	info, err := c.FetchProxiedIpInfo(ctx, socksPort)
	if err != nil {
		return "", err
	}

	if expectedCountry != "" && !strings.EqualFold(expectedCountry, "REC") && !strings.EqualFold(expectedCountry, "AUTO") {
		if !strings.EqualFold(info.CountryCode, expectedCountry) {
			return info.CountryCode, fmt.Errorf("exit country mismatch: expected %s, got %s", expectedCountry, info.CountryCode)
		}
	}
	return info.CountryCode, nil
}

func parseTrace(body string) (*IpLocationInfo, error) {
	scanner := bufio.NewScanner(strings.NewReader(body))
	var ip, loc string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ip=") {
			ip = strings.TrimSpace(strings.TrimPrefix(line, "ip="))
		} else if strings.HasPrefix(line, "loc=") {
			loc = strings.TrimSpace(strings.TrimPrefix(line, "loc="))
		}
	}

	if ip == "" && loc == "" {
		return nil, errors.New("trace response did not contain IP or location")
	}

	return &IpLocationInfo{
		IP:          ip,
		CountryCode: loc,
	}, nil
}
