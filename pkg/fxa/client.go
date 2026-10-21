package fxa

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	FxaAuthServer               = "https://api.accounts.firefox.com/v1"
	FirefoxClientID             = "5882386c6d801776"
	OAuthScope                  = "profile https://identity.mozilla.com/apps/vpn"
	MozillaVpnUserAgent         = "MozillaVPN/2.35.0 (sys:windows; iap:true)"
	VerificationMethodEmail2FA  = "email-2fa"
	ErrnoInvalidParameter       = 107
	MaxChallengeAttempts        = 5
	DefaultAccessTokenTTLSeconds = 24 * 3600
)

type SessionStatus int

const (
	SessionNeedsLogin SessionStatus = iota
	SessionActive
	SessionUnreachable
)

type FxaError struct {
	Message    string `json:"message"`
	Errno      int    `json:"errno,omitempty"`
	StatusCode int    `json:"code,omitempty"`
}

func (e *FxaError) Error() string {
	if e.Errno != 0 {
		return fmt.Sprintf("FxA error (status %d, errno %d): %s", e.StatusCode, e.Errno, e.Message)
	}
	return fmt.Sprintf("FxA error (status %d): %s", e.StatusCode, e.Message)
}

// RuntimeAuth represents persisted authentication tokens.
type RuntimeAuth struct {
	Email                 string `json:"email,omitempty"`
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token,omitempty"`
	ExpiresAtEpochSeconds int64  `json:"expires_at_epoch_seconds"`
	PendingSessionToken   string `json:"pending_session_token,omitempty"`
}

// GetSavedEmail returns the logged-in user email.
func (c *FxaClient) GetSavedEmail() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.auth != nil {
		return c.auth.Email
	}
	return ""
}

// FxaClient manages authentication, session tokens, and 2FA with Firefox Accounts.
type FxaClient struct {
	mu              sync.RWMutex
	httpClient      *http.Client
	challengeSolver *FastlyChallengeSolver
	authStorePath   string
	auth            *RuntimeAuth
}

// NewFxaClient creates a new FxA client with local storage persistence.
func NewFxaClient(storageDir string) (*FxaClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	solver := NewFastlyChallengeSolver(jar)

	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	storePath := filepath.Join(storageDir, "fxa_auth.json")
	fc := &FxaClient{
		httpClient:      client,
		challengeSolver: solver,
		authStorePath:   storePath,
	}

	_ = fc.loadAuth()
	return fc, nil
}

func (c *FxaClient) loadAuth() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.authStorePath)
	if err != nil {
		return err
	}

	var auth RuntimeAuth
	if err := json.Unmarshal(data, &auth); err != nil {
		return err
	}
	c.auth = &auth
	return nil
}

func (c *FxaClient) saveAuth(auth *RuntimeAuth) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.auth = auth
	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.authStorePath, data, 0600)
}

// HasValidAccessToken returns true if an access token exists and has not expired.
func (c *FxaClient) HasValidAccessToken() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.auth == nil || c.auth.AccessToken == "" {
		return false
	}
	return time.Now().Unix() < c.auth.ExpiresAtEpochSeconds-60
}

// StartLogin initiates sign-in using email and password.
// Returns needsTwoFactor = true if 2FA code is required to complete login.
func (c *FxaClient) StartLogin(email, password string) (bool, error) {
	cleanEmail := NormalizeEmail(email)
	authPw, err := DeriveAuthPw(cleanEmail, password)
	if err != nil {
		return false, fmt.Errorf("failed to derive authPW: %w", err)
	}

	respData, err := c.loginAttempt(cleanEmail, authPw, true)
	if err != nil {
		return false, err
	}

	sessionToken, _ := respData["sessionToken"].(string)
	if sessionToken == "" {
		return false, errors.New("FxA login response did not contain sessionToken")
	}

	verified, _ := respData["verified"].(bool)
	if !verified {
		c.mu.Lock()
		if c.auth == nil {
			c.auth = &RuntimeAuth{}
		}
		c.auth.Email = cleanEmail
		c.auth.PendingSessionToken = sessionToken
		c.mu.Unlock()
		_ = c.saveAuth(c.auth)
		return true, nil
	}

	c.mu.Lock()
	if c.auth == nil {
		c.auth = &RuntimeAuth{}
	}
	c.auth.Email = cleanEmail
	c.mu.Unlock()

	if err := c.completeLogin(sessionToken); err != nil {
		return false, err
	}
	return false, nil
}

func (c *FxaClient) loginAttempt(email, authPw string, withVerificationMethod bool) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"email":  email,
		"authPW": authPw,
	}
	if withVerificationMethod {
		payload["verificationMethod"] = VerificationMethodEmail2FA
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	respBytes, err := c.fxaDo(http.MethodPost, "/account/login", "", bodyBytes)
	if err != nil {
		var fxaErr *FxaError
		if errors.As(err, &fxaErr) && fxaErr.Errno == ErrnoInvalidParameter && withVerificationMethod {
			return c.loginAttempt(email, authPw, false)
		}
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to decode login response: %w", err)
	}
	return result, nil
}

// SubmitTwoFactorCode verifies the 2FA code and exchanges the session token for an OAuth token.
func (c *FxaClient) SubmitTwoFactorCode(code string) error {
	cleanCode := SanitizeCode(code)

	c.mu.RLock()
	var sessionToken string
	if c.auth != nil {
		sessionToken = c.auth.PendingSessionToken
	}
	c.mu.RUnlock()

	if sessionToken == "" {
		return errors.New("no pending FxA session found; please start sign-in again")
	}

	verifyPayload := map[string]interface{}{
		"code": cleanCode,
	}
	bodyBytes, err := json.Marshal(verifyPayload)
	if err != nil {
		return err
	}

	if _, err := c.fxaDo(http.MethodPost, "/session/verify_code", sessionToken, bodyBytes); err != nil {
		return fmt.Errorf("2FA verification failed: %w", err)
	}

	return c.completeLogin(sessionToken)
}

func (c *FxaClient) completeLogin(sessionToken string) error {
	oauthPayload := map[string]interface{}{
		"client_id":   FirefoxClientID,
		"grant_type":  "fxa-credentials",
		"scope":       OAuthScope,
		"access_type": "offline",
	}
	bodyBytes, err := json.Marshal(oauthPayload)
	if err != nil {
		return err
	}

	respBytes, err := c.fxaDo(http.MethodPost, "/oauth/token", sessionToken, bodyBytes)
	if err != nil {
		return fmt.Errorf("failed to fetch OAuth token: %w", err)
	}

	var tokenData struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(respBytes, &tokenData); err != nil {
		return fmt.Errorf("failed to parse OAuth token response: %w", err)
	}

	expiresAt := time.Now().Unix() + DefaultAccessTokenTTLSeconds
	if tokenData.ExpiresIn > 0 {
		expiresAt = time.Now().Unix() + tokenData.ExpiresIn
	}

	renewed := &RuntimeAuth{
		AccessToken:           tokenData.AccessToken,
		RefreshToken:          tokenData.RefreshToken,
		ExpiresAtEpochSeconds: expiresAt,
		PendingSessionToken:   "",
	}
	return c.saveAuth(renewed)
}

// EnsureFreshAccessToken checks the validity of current tokens and refreshes them if needed.
func (c *FxaClient) EnsureFreshAccessToken() (string, error) {
	c.mu.RLock()
	auth := c.auth
	c.mu.RUnlock()

	if auth == nil {
		return "", errors.New("not signed in to Firefox Accounts")
	}

	if c.HasValidAccessToken() {
		return auth.AccessToken, nil
	}

	if auth.RefreshToken == "" {
		return "", errors.New("stored session has no refresh token; sign in again")
	}

	refreshPayload := map[string]interface{}{
		"client_id":     FirefoxClientID,
		"grant_type":    "refresh_token",
		"refresh_token": auth.RefreshToken,
		"scope":         OAuthScope,
	}
	bodyBytes, err := json.Marshal(refreshPayload)
	if err != nil {
		return "", err
	}

	respBytes, err := c.fxaDo(http.MethodPost, "/oauth/token", "", bodyBytes)
	if err != nil {
		var fxaErr *FxaError
		if errors.As(err, &fxaErr) && (fxaErr.StatusCode == 400 || fxaErr.StatusCode == 401 || fxaErr.StatusCode == 403) {
			_ = c.SignOut()
			return "", fmt.Errorf("refresh token rejected permanently: %w", err)
		}
		return "", fmt.Errorf("refresh token request failed: %w", err)
	}

	var tokenData struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(respBytes, &tokenData); err != nil {
		return "", fmt.Errorf("failed to parse refreshed token response: %w", err)
	}

	refToken := tokenData.RefreshToken
	if refToken == "" {
		refToken = auth.RefreshToken
	}

	expiresAt := time.Now().Unix() + DefaultAccessTokenTTLSeconds
	if tokenData.ExpiresIn > 0 {
		expiresAt = time.Now().Unix() + tokenData.ExpiresIn
	}

	renewed := &RuntimeAuth{
		AccessToken:           tokenData.AccessToken,
		RefreshToken:          refToken,
		ExpiresAtEpochSeconds: expiresAt,
	}
	if err := c.saveAuth(renewed); err != nil {
		return "", fmt.Errorf("failed to save renewed auth: %w", err)
	}

	return renewed.AccessToken, nil
}

// RestoreSession checks if stored authentication can be restored or refreshed.
func (c *FxaClient) RestoreSession() (SessionStatus, error) {
	c.mu.RLock()
	auth := c.auth
	c.mu.RUnlock()

	if auth == nil || (auth.RefreshToken == "" && auth.AccessToken == "") {
		return SessionNeedsLogin, nil
	}

	if c.HasValidAccessToken() {
		return SessionActive, nil
	}

	_, err := c.EnsureFreshAccessToken()
	if err == nil {
		return SessionActive, nil
	}

	var fxaErr *FxaError
	if errors.As(err, &fxaErr) && (fxaErr.StatusCode == 400 || fxaErr.StatusCode == 401) {
		return SessionNeedsLogin, err
	}

	return SessionUnreachable, err
}

// SignOut clears local credentials.
func (c *FxaClient) SignOut() error {
	c.mu.Lock()
	c.auth = nil
	c.mu.Unlock()
	return os.Remove(c.authStorePath)
}

func (c *FxaClient) fxaDo(method, path string, sessionToken string, body []byte) ([]byte, error) {
	reqUrl, err := url.Parse(FxaAuthServer + path)
	if err != nil {
		return nil, fmt.Errorf("invalid FxA URL: %w", err)
	}

	buildRequest := func() (*http.Request, error) {
		req, err := http.NewRequest(method, reqUrl.String(), bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", MozillaVpnUserAgent)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")

		if sessionToken != "" {
			tokenId, hmacKey, err := DeriveHawkCredentials(sessionToken)
			if err != nil {
				return nil, fmt.Errorf("failed deriving Hawk credentials: %w", err)
			}
			hawkHdr, err := GenerateHawkHeader(method, reqUrl, tokenId, hmacKey, body)
			if err != nil {
				return nil, fmt.Errorf("failed generating Hawk header: %w", err)
			}
			req.Header.Set("Authorization", hawkHdr)
		}
		return req, nil
	}

	req, err := buildRequest()
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	attempts := 1
	for resp.StatusCode == 406 && attempts < MaxChallengeAttempts {
		_ = resp.Body.Close()
		if solveErr := c.challengeSolver.SolveAndInstall(); solveErr != nil {
			return nil, fmt.Errorf("fastly challenge solve failed: %w", solveErr)
		}
		req, err = buildRequest()
		if err != nil {
			return nil, err
		}
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		attempts++
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var fxaErr FxaError
		if err := json.Unmarshal(respBytes, &fxaErr); err == nil {
			fxaErr.StatusCode = resp.StatusCode
			return nil, &fxaErr
		}
		return nil, &FxaError{
			Message:    string(respBytes),
			StatusCode: resp.StatusCode,
		}
	}

	return respBytes, nil
}
