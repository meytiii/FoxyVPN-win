package fxa

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/pbkdf2"
)

const (
	ProtocolVersion = "identity.mozilla.com/picl/v1/"
	Pbkdf2Rounds    = 1000
	KeyLenBytes     = 32
)

// NormalizeEmail strips whitespace, zero-width characters, and converts to lowercase.
func NormalizeEmail(email string) string {
	var builder strings.Builder
	for _, r := range email {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\u00A0' ||
			r == '\u200B' || r == '\u200C' || r == '\u200D' || r == '\uFEFF' {
			continue
		}
		builder.WriteRune(r)
	}
	return strings.ToLower(builder.String())
}

// SanitizeCode removes whitespace from verification codes.
func SanitizeCode(code string) string {
	var builder strings.Builder
	for _, r := range code {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\u00A0' ||
			r == '\u200B' || r == '\u200C' || r == '\u200D' || r == '\uFEFF' {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

// DeriveQuickStretch computes PBKDF2-HMAC-SHA256 with Mozilla FxA salt.
func DeriveQuickStretch(email, password string) []byte {
	salt := []byte(ProtocolVersion + "quickStretch:" + email)
	return pbkdf2.Key([]byte(password), salt, Pbkdf2Rounds, KeyLenBytes, sha256.New)
}

// DeriveAuthPw computes the hex-encoded authPW for /account/login.
func DeriveAuthPw(email, password string) (string, error) {
	quickStretched := DeriveQuickStretch(email, password)
	salt := make([]byte, 32)
	kdf := hkdf.New(sha256.New, quickStretched, salt, []byte(ProtocolVersion+"authPW"))
	authPwBytes := make([]byte, 32)
	if _, err := io.ReadFull(kdf, authPwBytes); err != nil {
		return "", fmt.Errorf("failed to derive authPW via HKDF: %w", err)
	}
	return hex.EncodeToString(authPwBytes), nil
}

// DeriveHawkCredentials derives the 32-byte tokenId hex and 32-byte hmacKey from a session token hex.
func DeriveHawkCredentials(sessionTokenHex string) (string, []byte, error) {
	sessionTokenBytes, err := hex.DecodeString(sessionTokenHex)
	if err != nil {
		return "", nil, fmt.Errorf("invalid session token hex: %w", err)
	}
	salt := make([]byte, 32)
	kdf := hkdf.New(sha256.New, sessionTokenBytes, salt, []byte(ProtocolVersion+"sessionToken"))
	expanded := make([]byte, 64)
	if _, err := io.ReadFull(kdf, expanded); err != nil {
		return "", nil, fmt.Errorf("failed to derive Hawk credentials via HKDF: %w", err)
	}
	tokenId := hex.EncodeToString(expanded[0:32])
	hmacKey := expanded[32:64]
	return tokenId, hmacKey, nil
}

// GenerateHawkHeader computes the Hawk Authorization header value for an HTTP request.
func GenerateHawkHeader(method string, targetURL *url.URL, tokenId string, hmacKey []byte, body []byte) (string, error) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonceBytes := make([]byte, 6)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", fmt.Errorf("failed to generate random nonce: %w", err)
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)

	path := targetURL.EscapedPath()
	if path == "" {
		path = "/"
	}
	if targetURL.RawQuery != "" {
		path += "?" + targetURL.RawQuery
	}

	host := targetURL.Hostname()
	port := targetURL.Port()
	if port == "" {
		if strings.EqualFold(targetURL.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}

	payloadHash := ""
	if len(body) > 0 {
		h := sha256.New()
		h.Write([]byte("hawk.1.payload\napplication/json\n"))
		h.Write(body)
		h.Write([]byte("\n"))
		payloadHash = base64.StdEncoding.EncodeToString(h.Sum(nil))
	}

	normalized := fmt.Sprintf(
		"hawk.1.header\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n\n",
		ts,
		nonce,
		strings.ToUpper(method),
		path,
		host,
		port,
		payloadHash,
	)

	mac := hmac.New(sha256.New, hmacKey)
	mac.Write([]byte(normalized))
	macB64 := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	header := fmt.Sprintf(`Hawk id="%s", ts="%s", nonce="%s", mac="%s"`, tokenId, ts, nonce, macB64)
	if payloadHash != "" {
		header += fmt.Sprintf(`, hash="%s"`, payloadHash)
	}

	return header, nil
}
