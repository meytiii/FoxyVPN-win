package fxa

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	SolverUserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
	HtmlAccept           = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
	PowAlphabet          = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	MaxPostBackRounds    = 3
	SolveTimeoutDuration = 60 * time.Second
)

var (
	challengePrefixRegex = regexp.MustCompile(`/_fs-ch-[A-Za-z0-9]+`)
	initCallRegex        = regexp.MustCompile(`init\((\[[^\]]*\]),\s*"([^"]+)",\s*"([^"]+)"`)
)

// FastlyChallengeSolver handles Fastly Bot Protection challenges encountered by FxA endpoints.
type FastlyChallengeSolver struct {
	mu  sync.Mutex
	jar *cookiejar.Jar
}

// NewFastlyChallengeSolver constructs a new solver with a shared cookie jar.
func NewFastlyChallengeSolver(jar *cookiejar.Jar) *FastlyChallengeSolver {
	return &FastlyChallengeSolver{
		jar: jar,
	}
}

// SolveAndInstall attempts to solve challenges on Firefox Accounts host endpoints and saves cookies.
func (s *FastlyChallengeSolver) SolveAndInstall() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lastErr error
	hosts := []string{"https://api.accounts.firefox.com", "https://accounts.firefox.com"}
	for _, base := range hosts {
		err := s.solveOnHost(base)
		if err == nil {
			return nil
		}
		if errors.Is(err, errNoChallengePage) {
			continue
		}
		lastErr = err
	}

	if lastErr != nil {
		return lastErr
	}
	return errors.New("host did not serve a solvable Fastly challenge page")
}

var errNoChallengePage = errors.New("no challenge page on host")

func (s *FastlyChallengeSolver) solveOnHost(base string) error {
	client := &http.Client{
		Jar:     s.jar,
		Timeout: SolveTimeoutDuration,
	}

	pageUrl := base + "/"
	page, isChallenge, err := s.fetchChallengePage(client, pageUrl)
	if err != nil {
		return err
	}
	if !isChallenge {
		return errNoChallengePage
	}

	prefixMatch := challengePrefixRegex.FindString(page)
	if prefixMatch == "" {
		return fmt.Errorf("challenge asset prefix not found on %s", base)
	}
	prefixUrl := base + prefixMatch

	script, err := s.fetchText(client, prefixUrl+"/script.js?reload=true", HtmlAccept)
	if err != nil {
		return fmt.Errorf("failed to fetch challenge script: %w", err)
	}

	challenges, token, err := parseChallengeInit(script)
	if err != nil {
		return err
	}

	for round := 0; round < MaxPostBackRounds; round++ {
		var answers []map[string]interface{}
		for _, ch := range challenges {
			ans, err := s.answerChallenge(client, prefixUrl, token, ch)
			if err != nil {
				return fmt.Errorf("failed answering challenge: %w", err)
			}
			answers = append(answers, ans)
		}

		postPayload := map[string]interface{}{
			"token": token,
			"data":  answers,
		}
		postBody, err := json.Marshal(postPayload)
		if err != nil {
			return fmt.Errorf("failed to encode challenge answers: %w", err)
		}

		req, err := http.NewRequest(http.MethodPost, prefixUrl+"/fst-post-back", bytes.NewReader(postBody))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", SolverUserAgent)
		req.Header.Set("Origin", baseOrigin(prefixUrl))

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("challenge post-back error: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("challenge post-back returned HTTP %d", resp.StatusCode)
		}

		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read post-back response: %w", err)
		}

		var respData struct {
			Status string                   `json:"status"`
			Ch     []map[string]interface{} `json:"ch"`
			Tok    string                   `json:"tok"`
		}
		if err := json.Unmarshal(respBytes, &respData); err != nil {
			return fmt.Errorf("could not decode post-back response: %w", err)
		}

		if respData.Status == "success" {
			_, stillChallenged, _ := s.fetchChallengePage(client, pageUrl)
			if stillChallenged {
				return errors.New("challenge cookie not accepted on this exit IP")
			}
			return nil
		}

		if len(respData.Ch) == 0 || respData.Tok == "" {
			return fmt.Errorf("unexpected post-back response: %s", string(respBytes))
		}
		challenges = respData.Ch
		token = respData.Tok
	}

	return fmt.Errorf("Fastly challenge did not complete within %d rounds", MaxPostBackRounds)
}

func (s *FastlyChallengeSolver) fetchText(client *http.Client, targetUrl, accept string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, targetUrl, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", SolverUserAgent)
	req.Header.Set("Accept", accept)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GET %s returned HTTP %d", targetUrl, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

func (s *FastlyChallengeSolver) fetchChallengePage(client *http.Client, pageUrl string) (string, bool, error) {
	body, err := s.fetchText(client, pageUrl, HtmlAccept)
	if err != nil {
		return "", false, err
	}
	isChallenge := strings.Contains(body, "/_fs-ch-") && strings.Contains(body, "Client Challenge")
	return body, isChallenge, nil
}

func parseChallengeInit(script string) ([]map[string]interface{}, string, error) {
	matches := initCallRegex.FindAllStringSubmatch(script, -1)
	if len(matches) == 0 {
		return nil, "", errors.New("challenge init() call not found in script")
	}
	lastMatch := matches[len(matches)-1]
	jsonList := lastMatch[1]
	token := lastMatch[2]

	var challenges []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonList), &challenges); err != nil {
		return nil, "", fmt.Errorf("could not parse challenge list: %w", err)
	}
	if len(challenges) == 0 {
		return nil, "", errors.New("empty challenge list")
	}
	return challenges, token, nil
}

func (s *FastlyChallengeSolver) answerChallenge(client *http.Client, prefixUrl, token string, ch map[string]interface{}) (map[string]interface{}, error) {
	ty, _ := ch["ty"].(string)
	data, _ := ch["data"].(map[string]interface{})

	switch ty {
	case "pow":
		base, _ := data["base"].(string)
		targetHex, _ := data["hash"].(string)
		ans, err := solvePow(base, targetHex)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"ty":      "pow",
			"base":    base,
			"answer":  ans,
			"hmac":    data["hmac"],
			"expires": data["expires"],
		}, nil

	case "pat":
		patAuth, err := s.fetchPat(client, prefixUrl, token)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"ty":   "pat",
			"auth": patAuth,
		}, nil

	case "clientmetrics":
		return map[string]interface{}{
			"ty":        "clientmetrics",
			"webdriver": false,
			"bot_detection_result": map[string]interface{}{
				"bot_detected": false,
				"bot_kind":     nil,
			},
			"browser_metrics": map[string]interface{}{
				"client_data": "{}",
				"error_trace": nil,
			},
			"detector_results": map[string]interface{}{},
			"v":                2,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported Fastly challenge type: %s", ty)
	}
}

func (s *FastlyChallengeSolver) fetchPat(client *http.Client, prefixUrl, token string) (string, error) {
	patUrl := fmt.Sprintf("%s/pat?token=%s", prefixUrl, url.QueryEscape(token))
	req, err := http.NewRequest(http.MethodPost, patUrl, bytes.NewReader(nil))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", SolverUserAgent)
	req.Header.Set("Origin", baseOrigin(prefixUrl))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 400 || resp.StatusCode == 401 {
		return "", nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("PAT request returned HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var jsonResp struct {
		Auth string `json:"auth"`
	}
	if err := json.Unmarshal(bodyBytes, &jsonResp); err != nil {
		return "", fmt.Errorf("could not parse PAT response: %w", err)
	}
	if jsonResp.Auth == "" {
		return "", errors.New("empty PAT auth token")
	}
	return jsonResp.Auth, nil
}

func solvePow(base, targetHex string) (string, error) {
	target, err := hex.DecodeString(targetHex)
	if err != nil || len(target) != 32 {
		return "", fmt.Errorf("invalid target hex hash: %s", targetHex)
	}

	alphabetRunes := []rune(PowAlphabet)
	for _, a := range alphabetRunes {
		for _, b := range alphabetRunes {
			suffix := string([]rune{a, b})
			h := sha256.Sum256([]byte(base + suffix))
			if bytes.Equal(h[:], target) {
				return suffix, nil
			}
		}
	}
	return "", fmt.Errorf("no proof-of-work solution found for base %s", base)
}

func baseOrigin(prefixUrl string) string {
	idx := strings.Index(prefixUrl, "/_fs-ch-")
	if idx > 0 {
		return prefixUrl[:idx]
	}
	return prefixUrl
}
