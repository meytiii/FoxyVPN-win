package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"foxyvpn/pkg/fxa"
	"foxyvpn/pkg/guardian"
	"foxyvpn/pkg/proxy"
	"foxyvpn/pkg/remotesettings"
	"foxyvpn/pkg/upstream"
	"foxyvpn/pkg/winproxy"
)

type VpnState string

const (
	StateDisconnected VpnState = "DISCONNECTED"
	StateConnecting   VpnState = "CONNECTING"
	StateConnected    VpnState = "CONNECTED"
	StateReconnecting VpnState = "RECONNECTING"
	StateError        VpnState = "ERROR"
)

type VpnStatus struct {
	State               VpnState                       `json:"state"`
	ErrorMessage        string                         `json:"error_message,omitempty"`
	SelectedCountry     string                         `json:"selected_country"`
	SelectedCity        string                         `json:"selected_city"`
	ConnectedNode       string                         `json:"connected_node"`
	PingMs              int64                          `json:"ping_ms"`
	UploadSpeedBytes    int64                          `json:"upload_speed_bytes"`
	DownloadSpeedBytes  int64                          `json:"download_speed_bytes"`
	TotalUploadedBytes  int64                          `json:"total_uploaded_bytes"`
	TotalDownloadedBytes int64                         `json:"total_downloaded_bytes"`
	QuotaLimitBytes     int64                          `json:"quota_limit_bytes"`
	QuotaRemainingBytes int64                          `json:"quota_remaining_bytes"`
	QuotaResetEpoch     int64                          `json:"quota_reset_epoch"`
	IsSignedIn          bool                           `json:"is_signed_in"`
	TlsSplitMode        int                            `json:"tls_split_mode"`
	SystemProxyEnabled  bool                           `json:"system_proxy_enabled"`
	SocksPort           int                            `json:"socks_port"`
	HttpPort            int                            `json:"http_port"`
}

type VpnService struct {
	mu                  sync.RWMutex
	state               VpnState
	errorMessage        string
	selectedCountry     string
	selectedCity        string
	connectedNode       string
	socksPort           int
	httpPort            int
	tlsSplitMode        upstream.TlsSplitMode
	enableSystemProxy   bool

	fxaClient           *fxa.FxaClient
	guardianClient      *guardian.GuardianClient
	serverRepo          *remotesettings.ServerRepository
	upstreamManager     *upstream.UpstreamManager
	socksServer         *proxy.Socks5Server
	httpServer          *proxy.HttpProxyServer
	sysProxy            *winproxy.SystemProxyManager
	trafficStats        *proxy.TrafficStats

	lastUploadSample    int64
	lastDownloadSample  int64
	uploadSpeedBps      int64
	downloadSpeedBps    int64

	proxyPass           *guardian.ProxyPass
	statusListeners     []func(status VpnStatus)
	ctx                 context.Context
	cancel              context.CancelFunc
}

func NewVpnService() (*VpnService, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = "."
	}
	appDir := filepath.Join(appData, "FoxyVPN")
	_ = os.MkdirAll(appDir, 0700)

	fc, err := fxa.NewFxaClient(appDir)
	if err != nil {
		return nil, fmt.Errorf("failed to init FxA client: %w", err)
	}

	gc, err := guardian.NewGuardianClient("")
	if err != nil {
		return nil, fmt.Errorf("failed to init Guardian client: %w", err)
	}

	sRepo := remotesettings.NewServerRepository(appDir)

	stats := &proxy.TrafficStats{}
	sysProxy := winproxy.NewSystemProxyManager()

	service := &VpnService{
		state:              StateDisconnected,
		selectedCountry:    "US",
		selectedCity:       "nyc",
		socksPort:          1080,
		httpPort:           8080,
		tlsSplitMode:       upstream.SplitFirstByte,
		enableSystemProxy:  true,
		fxaClient:          fc,
		guardianClient:     gc,
		serverRepo:         sRepo,
		sysProxy:           sysProxy,
		trafficStats:       stats,
	}

	upstreamMgr := upstream.NewUpstreamManager(func() (string, error) {
		return service.getGuardianToken()
	})
	upstreamMgr.SetSplitMode(service.tlsSplitMode)
	service.upstreamManager = upstreamMgr

	service.socksServer = proxy.NewSocks5Server(func(address string) (net.Conn, error) {
		return upstreamMgr.Dial(address)
	}, stats)

	service.httpServer = proxy.NewHttpProxyServer(func(address string) (net.Conn, error) {
		return upstreamMgr.Dial(address)
	}, stats)

	go service.meteringLoop()

	return service, nil
}

func (s *VpnService) RegisterListener(listener func(status VpnStatus)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusListeners = append(s.statusListeners, listener)
}

func (s *VpnService) notifyListeners() {
	status := s.GetStatus()
	s.mu.RLock()
	listeners := append([]func(status VpnStatus){}, s.statusListeners...)
	s.mu.RUnlock()

	for _, l := range listeners {
		l(status)
	}
}

func (s *VpnService) getGuardianToken() (string, error) {
	s.mu.RLock()
	cached := s.proxyPass
	s.mu.RUnlock()

	if cached != nil && cached.Token != "" && time.Now().Unix() < cached.ExpiresAtEpochSeconds-120 {
		return cached.Token, nil
	}

	accessToken, err := s.fxaClient.EnsureFreshAccessToken()
	if err != nil {
		return "", fmt.Errorf("authentication expired: %w", err)
	}

	pass, err := s.guardianClient.FetchProxyPass(accessToken)
	if err != nil {
		return "", fmt.Errorf("failed to fetch guardian proxy pass: %w", err)
	}

	s.mu.Lock()
	s.proxyPass = pass
	s.mu.Unlock()
	return pass.Token, nil
}

func (s *VpnService) Connect() error {
	s.mu.Lock()
	if s.state == StateConnected || s.state == StateConnecting {
		s.mu.Unlock()
		return nil
	}
	s.state = StateConnecting
	s.errorMessage = ""
	country := s.selectedCountry
	s.mu.Unlock()
	s.notifyListeners()

	go func() {
		err := s.executeConnect(country)
		s.mu.Lock()
		if err != nil {
			s.state = StateError
			s.errorMessage = err.Error()
			_ = s.executeDisconnect()
		} else {
			s.state = StateConnected
			s.errorMessage = ""
		}
		s.mu.Unlock()
		s.notifyListeners()
	}()

	return nil
}

func (s *VpnService) executeConnect(countryCode string) error {
	// Step 1: Ensure valid FxA tokens
	_, err := s.fxaClient.EnsureFreshAccessToken()
	if err != nil {
		return fmt.Errorf("FxA sign-in required: %w", err)
	}

	// Step 2: Ensure Guardian pass & quota
	pass, err := s.getGuardianToken()
	if err != nil || pass == "" {
		return fmt.Errorf("could not obtain Guardian token: %w", err)
	}

	// Step 3: Find candidates for selected country
	countries, err := s.serverRepo.FetchCountries()
	if err != nil || len(countries) == 0 {
		countries = remotesettings.BuiltinFallbackCountries()
	}

	candidates := remotesettings.CandidatesForCountry(countries, countryCode)
	if len(candidates) == 0 {
		candidates = remotesettings.CandidatesForCountry(countries, "")
	}
	if len(candidates) == 0 {
		return errors.New("no proxy candidates available for country")
	}

	var connectedErr error
	var connectedHost string
	var connectedPort int

	for _, cand := range candidates {
		s.upstreamManager.SetSplitMode(s.tlsSplitMode)
		if dialErr := s.upstreamManager.Connect(cand.Host, cand.Port); dialErr == nil {
			connectedHost = cand.Host
			connectedPort = cand.Port
			connectedErr = nil
			break
		} else {
			connectedErr = dialErr
		}
	}

	if connectedErr != nil {
		return fmt.Errorf("failed connecting to edge proxies: %w", connectedErr)
	}

	s.mu.Lock()
	s.connectedNode = fmt.Sprintf("%s:%d", connectedHost, connectedPort)
	s.mu.Unlock()

	// Step 4: Start local SOCKS5 & HTTP proxies
	socksAddr := fmt.Sprintf("127.0.0.1:%d", s.socksPort)
	if err := s.socksServer.Start(socksAddr); err != nil {
		return fmt.Errorf("failed to start SOCKS5 proxy: %w", err)
	}

	httpAddr := fmt.Sprintf("127.0.0.1:%d", s.httpPort)
	if err := s.httpServer.Start(httpAddr); err != nil {
		_ = s.socksServer.Stop()
		return fmt.Errorf("failed to start HTTP proxy: %w", err)
	}

	// Step 5: Toggle Windows System Proxy if enabled
	if s.enableSystemProxy {
		if err := s.sysProxy.Enable(httpAddr, socksAddr); err != nil {
			// Log warning but don't abort tunnel
			fmt.Printf("Warning: failed setting system proxy: %v\n", err)
		}
	}

	return nil
}

func (s *VpnService) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = StateDisconnected
	s.errorMessage = ""
	err := s.executeDisconnect()
	s.notifyListeners()
	return err
}

func (s *VpnService) executeDisconnect() error {
	if s.sysProxy != nil {
		_ = s.sysProxy.Disable()
	}
	if s.httpServer != nil {
		_ = s.httpServer.Stop()
	}
	if s.socksServer != nil {
		_ = s.socksServer.Stop()
	}
	if s.upstreamManager != nil {
		_ = s.upstreamManager.Close()
	}
	s.connectedNode = ""
	return nil
}

func (s *VpnService) SetSelectedCountry(code, city string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.selectedCountry = code
	s.selectedCity = city
}

func (s *VpnService) SetTlsSplitMode(mode int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tlsSplitMode = upstream.TlsSplitMode(mode)
	if s.upstreamManager != nil {
		s.upstreamManager.SetSplitMode(s.tlsSplitMode)
	}
}

func (s *VpnService) SetSystemProxyEnabled(enable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enableSystemProxy = enable
	if !enable && s.sysProxy != nil {
		_ = s.sysProxy.Disable()
	} else if enable && s.state == StateConnected && s.sysProxy != nil {
		httpAddr := fmt.Sprintf("127.0.0.1:%d", s.httpPort)
		socksAddr := fmt.Sprintf("127.0.0.1:%d", s.socksPort)
		_ = s.sysProxy.Enable(httpAddr, socksAddr)
	}
}

func (s *VpnService) meteringLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		curUp := s.trafficStats.BytesUploaded.Load()
		curDown := s.trafficStats.BytesDownloaded.Load()

		s.uploadSpeedBps = curUp - s.lastUploadSample
		s.downloadSpeedBps = curDown - s.lastDownloadSample

		s.lastUploadSample = curUp
		s.lastDownloadSample = curDown
		s.mu.Unlock()

		if s.state == StateConnected {
			s.notifyListeners()
		}
	}
}

func (s *VpnService) GetStatus() VpnStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ping int64
	if s.upstreamManager != nil {
		ping = s.upstreamManager.Ping()
	}

	var quotaLimit, quotaRemaining, quotaReset int64
	if s.proxyPass != nil {
		quotaLimit = s.proxyPass.QuotaLimitBytes
		quotaRemaining = s.proxyPass.QuotaRemainingBytes
		quotaReset = s.proxyPass.QuotaResetEpoch
	}

	return VpnStatus{
		State:               s.state,
		ErrorMessage:        s.errorMessage,
		SelectedCountry:     s.selectedCountry,
		SelectedCity:        s.selectedCity,
		ConnectedNode:       s.connectedNode,
		PingMs:              ping,
		UploadSpeedBytes:    s.uploadSpeedBps,
		DownloadSpeedBytes:  s.downloadSpeedBps,
		TotalUploadedBytes:  s.trafficStats.BytesUploaded.Load(),
		TotalDownloadedBytes: s.trafficStats.BytesDownloaded.Load(),
		QuotaLimitBytes:     quotaLimit,
		QuotaRemainingBytes: quotaRemaining,
		QuotaResetEpoch:     quotaReset,
		IsSignedIn:          s.fxaClient.HasValidAccessToken(),
		TlsSplitMode:        int(s.tlsSplitMode),
		SystemProxyEnabled:  s.enableSystemProxy,
		SocksPort:           s.socksPort,
		HttpPort:            s.httpPort,
	}
}

// FxaAuth methods
func (s *VpnService) FxaLogin(email, password string) (bool, error) {
	needs2FA, err := s.fxaClient.StartLogin(email, password)
	if err == nil && !needs2FA {
		_, _ = s.getGuardianToken()
	}
	s.notifyListeners()
	return needs2FA, err
}

func (s *VpnService) FxaSubmit2FACode(code string) error {
	err := s.fxaClient.SubmitTwoFactorCode(code)
	if err == nil {
		_, _ = s.getGuardianToken()
	}
	s.notifyListeners()
	return err
}

func (s *VpnService) FxaSignOut() error {
	_ = s.Disconnect()
	s.mu.Lock()
	s.proxyPass = nil
	s.mu.Unlock()
	err := s.fxaClient.SignOut()
	s.notifyListeners()
	return err
}

func (s *VpnService) RestoreSession() (int, error) {
	status, err := s.fxaClient.RestoreSession()
	if status == fxa.SessionActive {
		_, _ = s.getGuardianToken()
	}
	s.notifyListeners()
	return int(status), err
}

func (s *VpnService) GetCountries() ([]remotesettings.VpnCountry, error) {
	return s.serverRepo.FetchCountries()
}
