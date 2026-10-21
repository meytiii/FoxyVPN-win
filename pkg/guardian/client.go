package guardian

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"sync"
	"time"

	"foxyvpn/pkg/fxa"
)

const (
	DefaultGuardianEndpoint = "https://vpn.mozilla.org"
	MozillaVpnUserAgent     = "MozillaVPN/2.35.0 (sys:windows; iap:true)"
	MaxChallengeAttempts    = 5
)

// ProxyPass contains the Guardian JWT token and current account quota metadata.
type ProxyPass struct {
	Token                 string `json:"token"`
	ExpiresAtEpochSeconds int64  `json:"expires_at_epoch_seconds"`
	QuotaLimitBytes       int64  `json:"quota_limit_bytes"`
	QuotaRemainingBytes   int64  `json:"quota_remaining_bytes"`
	QuotaResetEpoch       int64  `json:"quota_reset_epoch"`
}

// Entitlement represents Mozilla VPN account subscription and quota status.
type Entitlement struct {
	Subscribed          bool   `json:"subscribed"`
	UID                 string `json:"uid"`
	MaxBytes            int64  `json:"max_bytes"`
	LimitedBandwidth    bool   `json:"limited_bandwidth"`
	QuotaRemainingBytes int64  `json:"quota_remaining_bytes"`
}

// GuardianClient communicates with Mozilla VPN's Guardian API.
type GuardianClient struct {
	mu              sync.RWMutex
	endpoint        string
	httpClient      *http.Client
	challengeSolver *fxa.FastlyChallengeSolver
	cachedPass      *ProxyPass
}

// NewGuardianClient creates a new Guardian client.
func NewGuardianClient(endpoint string) (*GuardianClient, error) {
	if endpoint == "" {
		endpoint = DefaultGuardianEndpoint
	}
	endpoint = strings.TrimRight(endpoint, "/")

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create guardian cookie jar: %w", err)
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	solver := fxa.NewFastlyChallengeSolver(jar)

	return &GuardianClient{
		endpoint:        endpoint,
		httpClient:      client,
		challengeSolver: solver,
	}, nil
}

// FetchProxyPass obtains a fresh Guardian proxy token using the FxA OAuth access token.
func (g *GuardianClient) FetchProxyPass(accessToken string) (*ProxyPass, error) {
	url := g.endpoint + "/api/v1/fpn/token"
	resp, err := g.authorizedRequest(http.MethodGet, url, accessToken, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, errors.New("access token rejected by Guardian; sign-in refresh required")
	}
	if resp.StatusCode == 429 {
		return nil, errors.New("monthly free VPN data quota exceeded")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("guardian error HTTP %d: %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read proxy pass response: %w", err)
	}

	var jsonBody struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := json.Unmarshal(bodyBytes, &jsonBody); err != nil {
		return nil, fmt.Errorf("failed to decode proxy pass JSON: %w", err)
	}

	if jsonBody.Token == "" {
		return nil, errors.New("proxy pass response did not contain token")
	}

	expiresAt := jsonBody.ExpiresAt
	if expiresAt <= 0 {
		expiresAt = parseJwtExpiry(jsonBody.Token)
	}

	limit, _ := strconv.ParseInt(resp.Header.Get("X-Quota-Limit"), 10, 64)
	remaining, _ := strconv.ParseInt(resp.Header.Get("X-Quota-Remaining"), 10, 64)
	reset, _ := strconv.ParseInt(resp.Header.Get("X-Quota-Reset"), 10, 64)

	pass := &ProxyPass{
		Token:                 jsonBody.Token,
		ExpiresAtEpochSeconds: expiresAt,
		QuotaLimitBytes:       limit,
		QuotaRemainingBytes:   remaining,
		QuotaResetEpoch:       reset,
	}

	g.mu.Lock()
	g.cachedPass = pass
	g.mu.Unlock()

	return pass, nil
}

// FetchUserInfo gets subscription and bandwidth status.
func (g *GuardianClient) FetchUserInfo(accessToken string) (*Entitlement, error) {
	url := g.endpoint + "/api/v1/fpn/status"
	resp, err := g.authorizedRequest(http.MethodGet, url, accessToken, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch status HTTP %d: %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Subscribed       bool   `json:"subscribed"`
		UID              string `json:"uid"`
		MaxBytes         *int64 `json:"maxBytes"`
		LimitedBandwidth bool   `json:"limited_bandwidth"`
	}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode entitlement JSON: %w", err)
	}

	ent := &Entitlement{
		Subscribed:       raw.Subscribed,
		UID:              raw.UID,
		LimitedBandwidth: raw.LimitedBandwidth,
	}
	if raw.MaxBytes != nil {
		ent.MaxBytes = *raw.MaxBytes
	}

	if ent.LimitedBandwidth {
		pass, passErr := g.FetchProxyPass(accessToken)
		if passErr == nil && pass != nil {
			ent.QuotaRemainingBytes = pass.QuotaRemainingBytes
		}
	}

	return ent, nil
}

// ActivateEntitlement activates free proxy entitlement if account is new.
func (g *GuardianClient) ActivateEntitlement(accessToken string) (*Entitlement, error) {
	url := g.endpoint + "/api/v1/fpn/activate"
	resp, err := g.authorizedRequest(http.MethodPost, url, accessToken, []byte("{}"))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to activate entitlement HTTP %d: %s", resp.StatusCode, string(body))
	}

	return g.FetchUserInfo(accessToken)
}

// EnsureValidToken returns a cached non-expired token or fetches a new one.
func (g *GuardianClient) EnsureValidToken(accessToken string) (string, error) {
	g.mu.RLock()
	pass := g.cachedPass
	g.mu.RUnlock()

	if pass != nil && pass.Token != "" && time.Now().Unix() < pass.ExpiresAtEpochSeconds-300 {
		return pass.Token, nil
	}

	newPass, err := g.FetchProxyPass(accessToken)
	if err != nil {
		return "", err
	}
	return newPass.Token, nil
}

func (g *GuardianClient) authorizedRequest(method, targetUrl, accessToken string, body []byte) (*http.Response, error) {
	buildReq := func() (*http.Request, error) {
		req, err := http.NewRequest(method, targetUrl, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", MozillaVpnUserAgent)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		return req, nil
	}

	req, err := buildReq()
	if err != nil {
		return nil, err
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	attempts := 1
	for resp.StatusCode == 406 && attempts < MaxChallengeAttempts {
		_ = resp.Body.Close()
		if solveErr := g.challengeSolver.SolveAndInstall(); solveErr != nil {
			return nil, fmt.Errorf("fastly challenge solve failed: %w", solveErr)
		}
		req, err = buildReq()
		if err != nil {
			return nil, err
		}
		resp, err = g.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		attempts++
	}

	return resp, nil
}

func parseJwtExpiry(token string) int64 {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Now().Unix() + 3600
	}

	segment := parts[1]
	switch len(segment) % 4 {
	case 2:
		segment += "=="
	case 3:
		segment += "="
	}

	payloadBytes, err := base64.URLEncoding.DecodeString(segment)
	if err != nil {
		return time.Now().Unix() + 3600
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil || claims.Exp <= 0 {
		return time.Now().Unix() + 3600
	}

	return claims.Exp
}
