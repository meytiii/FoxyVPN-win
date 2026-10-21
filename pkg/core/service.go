package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"foxyvpn/pkg/dns"
	"foxyvpn/pkg/exitcheck"
	"foxyvpn/pkg/fxa"
	"foxyvpn/pkg/guardian"
	"foxyvpn/pkg/logger"
	"foxyvpn/pkg/ping"
	"foxyvpn/pkg/proxy"
	"foxyvpn/pkg/remotesettings"
	"foxyvpn/pkg/settings"
	"foxyvpn/pkg/upstream"
	"foxyvpn/pkg/winproxy"
	"foxyvpn/pkg/winutil"
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
	State                VpnState `json:"state"`
	ErrorMessage         string   `json:"error_message,omitempty"`
	SelectedCountry      string   `json:"selected_country"`
	SelectedCity         string   `json:"selected_city"`
	ConnectedNode        string   `json:"connected_node"`
	PingMs               int64    `json:"ping_ms"`
	UploadSpeedBytes     int64    `json:"upload_speed_bytes"`
	DownloadSpeedBytes   int64    `json:"download_speed_bytes"`
	TotalUploadedBytes   int64    `json:"total_uploaded_bytes"`
	TotalDownloadedBytes int64    `json:"total_downloaded_bytes"`
	QuotaLimitBytes      int64    `json:"quota_limit_bytes"`
	QuotaRemainingBytes  int64    `json:"quota_remaining_bytes"`
	QuotaResetEpoch      int64    `json:"quota_reset_epoch"`
	IsUnlimited          bool     `json:"is_unlimited"`
	IsSignedIn           bool     `json:"is_signed_in"`
	UserEmail            string   `json:"user_email"`
	TlsSplitMode         int      `json:"tls_split_mode"`
	SystemProxyEnabled   bool     `json:"system_proxy_enabled"`
	SocksPort            int      `json:"socks_port"`
	HttpPort             int      `json:"http_port"`
	SocksBindAddress     string   `json:"socks_bind_address"`
	RealIP               string   `json:"real_ip"`
	RealCountry          string   `json:"real_country"`
	ExitIP               string   `json:"exit_ip"`
	ExitCountry          string   `json:"exit_country"`
	IsProtected          bool     `json:"is_protected"`
	ExitCheckEnabled     bool     `json:"exit_check_enabled"`
	DnsProvider          string   `json:"dns_provider"`
	CustomDohURL         string   `json:"custom_doh_url"`
	BootstrapDns         string   `json:"bootstrap_dns"`
	CustomBootstrapIP    string   `json:"custom_bootstrap_ip"`
	CustomEdgeAddress    string   `json:"custom_edge_address"`
	UpstreamProxyEnabled bool     `json:"upstream_proxy_enabled"`
	UpstreamProxyType    string   `json:"upstream_proxy_type"`
	UpstreamProxyHost    string   `json:"upstream_proxy_host"`
	UpstreamProxyPort    int      `json:"upstream_proxy_port"`
	MinimizeToTray       bool     `json:"minimize_to_tray"`
	CloseToTray          bool     `json:"close_to_tray"`
	LaunchOnStartup      bool     `json:"launch_on_startup"`
	BypassLan            bool     `json:"bypass_lan"`
	CustomBypassList     string   `json:"custom_bypass_list"`
}

type VpnService struct {
	mu                  sync.RWMutex
	state               VpnState
	errorMessage        string
	selectedCountry     string
	selectedCity        string
	connectedNode       string
	enableSystemProxy   bool

	realIP              string
	realCountry         string
	exitIP              string
	exitCountry         string
	isProtected         bool
	isUnlimited         bool

	settingsStore       *settings.SettingsStore
	pingManager         *ping.PingManager
	exitChecker         *exitcheck.ExitChecker
	dohClient           *dns.DohClient
	bootstrapResolver   *dns.BootstrapResolver

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
	appDir              string
}

func NewVpnService() (*VpnService, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = "."
	}
	appDir := filepath.Join(appData, "FoxyVPN")
	_ = os.MkdirAll(appDir, 0700)

	log := logger.GetLogger()
	log.Info("Core", "Initializing FoxyVPN desktop core service")

	settingsStore := settings.NewSettingsStore(appDir)
	s := settingsStore.Get()

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
	pingMgr := ping.NewPingManager(1500 * time.Millisecond)
	exitChk := exitcheck.NewExitChecker()
	doh := dns.NewDohClient(s.SocksPort)
	bResolver := dns.NewBootstrapResolver()

	service := &VpnService{
		state:              StateDisconnected,
		selectedCountry:    "REC",
		selectedCity:       "REC",
		enableSystemProxy:  true,
		settingsStore:      settingsStore,
		pingManager:        pingMgr,
		exitChecker:        exitChk,
		dohClient:          doh,
		bootstrapResolver:  bResolver,
		fxaClient:          fc,
		guardianClient:     gc,
		serverRepo:         sRepo,
		sysProxy:           sysProxy,
		trafficStats:       stats,
		appDir:             appDir,
	}

	upstreamMgr := upstream.NewUpstreamManager(func() (string, error) {
		return service.getGuardianToken()
	})
	upstreamMgr.SetSplitMode(upstream.TlsSplitMode(s.TlsSplitMode))
	service.upstreamManager = upstreamMgr

	service.socksServer = proxy.NewSocks5Server(func(address string) (net.Conn, error) {
		return upstreamMgr.Dial(address)
	}, stats)

	service.httpServer = proxy.NewHttpProxyServer(func(address string) (net.Conn, error) {
		return upstreamMgr.Dial(address)
	}, stats)

	// Configure initial DoH endpoint on SOCKS5 proxy
	service.updateDohConfiguration()

	go service.meteringLoop()
	go service.tokenRenewalLoop()
	go service.watchdogLoop()
	go service.fetchInitialDirectIP()

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

func (s *VpnService) fetchInitialDirectIP() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	info, err := s.exitChecker.FetchDirectIpInfo(ctx)
	if err == nil && info != nil {
		s.mu.Lock()
		s.realIP = info.IP
		s.realCountry = info.CountryCode
		s.mu.Unlock()
		s.notifyListeners()
		logger.GetLogger().Info("ExitCheck", fmt.Sprintf("Direct real IP detected: %s (%s)", info.IP, info.CountryCode))
	}
}

func (s *VpnService) updateDohConfiguration() {
	cfg := s.settingsStore.Get()
	var effectiveURL string

	switch strings.ToUpper(cfg.DnsProvider) {
	case "CLOUDFLARE":
		effectiveURL = "https://1.1.1.1/dns-query"
	case "GOOGLE":
		effectiveURL = "https://8.8.8.8/dns-query"
	case "QUAD9":
		effectiveURL = "https://9.9.9.9/dns-query"
	case "ADGUARD":
		effectiveURL = "https://dns.adguard-dns.com/dns-query"
	case "CUSTOM":
		effectiveURL = strings.TrimSpace(cfg.CustomDohURL)
	default:
		// SYSTEM_DEFAULT
		effectiveURL = ""
	}

	s.socksServer.SetDoh(s.dohClient, effectiveURL)
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
	city := s.selectedCity
	s.mu.Unlock()
	s.notifyListeners()

	go func() {
		err := s.executeConnect(country, city)
		s.mu.Lock()
		if err != nil {
			s.state = StateError
			s.errorMessage = err.Error()
			s.isProtected = false
			_ = s.executeDisconnect()
			logger.GetLogger().Error("Tunnel", fmt.Sprintf("Tunnel connection failed: %v", err))
		} else {
			s.state = StateConnected
			s.errorMessage = ""
			s.isProtected = true
			logger.GetLogger().Info("Tunnel", fmt.Sprintf("Tunnel successfully connected to %s", s.connectedNode))
			go s.verifyExitAfterConnect()
		}
		s.mu.Unlock()
		s.notifyListeners()
	}()

	return nil
}

func (s *VpnService) executeConnect(countryCode, cityCode string) error {
	cfg := s.settingsStore.Get()

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

	// Step 3: Find candidates for selected country/city
	countries, err := s.serverRepo.FetchCountries()
	if err != nil || len(countries) == 0 {
		countries = remotesettings.BuiltinFallbackCountries()
	}

	var candidates []remotesettings.ProxyCandidate
	if cityCode != "" && !strings.EqualFold(cityCode, "REC") {
		candidates = remotesettings.CandidatesForCity(countries, countryCode, cityCode)
	}
	if len(candidates) == 0 {
		candidates = remotesettings.CandidatesForCountry(countries, countryCode)
	}
	if len(candidates) == 0 {
		candidates = remotesettings.CandidatesForCountry(countries, "")
	}
	if len(candidates) == 0 {
		return errors.New("no proxy candidates available for country")
	}

	// Sort candidates or pick best
	var connectedErr error
	var connectedHost string
	var connectedPort int

	dialCfg := upstream.UpstreamDialConfig{
		CustomEdgeAddress:    cfg.CustomEdgeAddress,
		SplitMode:            upstream.TlsSplitMode(cfg.TlsSplitMode),
		UpstreamProxyEnabled: cfg.UpstreamProxyEnabled,
		UpstreamProxyType:    cfg.UpstreamProxyType,
		UpstreamProxyHost:    cfg.UpstreamProxyHost,
		UpstreamProxyPort:    cfg.UpstreamProxyPort,
		UpstreamProxyUser:    cfg.UpstreamProxyUsername,
		UpstreamProxyPass:    cfg.UpstreamProxyPassword,
	}

	// Try candidates in order
	for _, cand := range candidates {
		dialCfg.Host = cand.Host
		dialCfg.Port = cand.Port

		logger.GetLogger().Info("Tunnel", fmt.Sprintf("Dialing upstream edge %s:%d (%s - %s)...", cand.Host, cand.Port, cand.CountryCode, cand.CityName))
		if dialErr := s.upstreamManager.ConnectWithConfig(dialCfg); dialErr == nil {
			connectedHost = cand.Host
			connectedPort = cand.Port
			connectedErr = nil
			break
		} else {
			logger.GetLogger().Warn("Tunnel", fmt.Sprintf("Candidate %s:%d failed: %v", cand.Host, cand.Port, dialErr))
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
	s.dohClient.SetSocksPort(cfg.SocksPort)
	s.updateDohConfiguration()

	bindAddr := cfg.SocksBindAddress
	if bindAddr == "" {
		bindAddr = "127.0.0.1"
	}

	socksAddr := fmt.Sprintf("%s:%d", bindAddr, cfg.SocksPort)
	if err := s.socksServer.Start(socksAddr); err != nil {
		return fmt.Errorf("failed to start SOCKS5 proxy on %s: %w", socksAddr, err)
	}

	httpAddr := fmt.Sprintf("%s:%d", bindAddr, cfg.HttpProxyPort)
	if cfg.HttpProxyEnabled {
		if err := s.httpServer.Start(httpAddr); err != nil {
			_ = s.socksServer.Stop()
			return fmt.Errorf("failed to start HTTP proxy on %s: %w", httpAddr, err)
		}
	}

	// Step 5: Toggle Windows System Proxy if enabled and not in Proxy-Only mode
	if s.enableSystemProxy && !cfg.ProxyOnlyMode {
		localHttp := fmt.Sprintf("127.0.0.1:%d", cfg.HttpProxyPort)
		localSocks := fmt.Sprintf("127.0.0.1:%d", cfg.SocksPort)
		bypassList := cfg.CustomBypassList
		if cfg.BypassLan && !strings.Contains(bypassList, "192.168.*") {
			bypassList += ";192.168.*;10.*;172.16.*;<local>"
		}
		if err := s.sysProxy.Enable(localHttp, localSocks, bypassList); err != nil {
			logger.GetLogger().Warn("WinProxy", fmt.Sprintf("Failed setting Windows system proxy: %v", err))
		} else {
			logger.GetLogger().Info("WinProxy", "Windows WinINet system proxy enabled")
		}
	}

	return nil
}

func (s *VpnService) verifyExitAfterConnect() {
	cfg := s.settingsStore.Get()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	info, err := s.exitChecker.FetchProxiedIpInfo(ctx, cfg.SocksPort)
	if err == nil && info != nil {
		s.mu.Lock()
		s.exitIP = info.IP
		s.exitCountry = info.CountryCode
		s.isProtected = true
		s.mu.Unlock()
		s.notifyListeners()

		logger.GetLogger().Info("ExitCheck", fmt.Sprintf("Verified proxied egress IP: %s (%s)", info.IP, info.CountryCode))

		// Check exit country match if requested
		if cfg.ExitCheckEnabled && s.selectedCountry != "" && !strings.EqualFold(s.selectedCountry, "REC") {
			if !strings.EqualFold(info.CountryCode, s.selectedCountry) {
				logger.GetLogger().Warn("ExitCheck", fmt.Sprintf("Exit country mismatch: expected %s, got %s", s.selectedCountry, info.CountryCode))
			}
		}
	} else if err != nil {
		logger.GetLogger().Warn("ExitCheck", fmt.Sprintf("Could not verify proxied exit IP: %v", err))
	}
}

func (s *VpnService) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = StateDisconnected
	s.errorMessage = ""
	s.isProtected = false
	err := s.executeDisconnect()
	s.notifyListeners()
	logger.GetLogger().Info("Tunnel", "Tunnel disconnected")
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
	cfg := s.settingsStore.Get()
	cfg.TlsSplitMode = mode
	_ = s.settingsStore.Save(cfg)

	if s.upstreamManager != nil {
		s.upstreamManager.SetSplitMode(upstream.TlsSplitMode(mode))
	}
}

func (s *VpnService) SetSystemProxyEnabled(enable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enableSystemProxy = enable

	if !enable && s.sysProxy != nil {
		_ = s.sysProxy.Disable()
	} else if enable && s.state == StateConnected && s.sysProxy != nil {
		cfg := s.settingsStore.Get()
		httpAddr := fmt.Sprintf("127.0.0.1:%d", cfg.HttpProxyPort)
		socksAddr := fmt.Sprintf("127.0.0.1:%d", cfg.SocksPort)
		_ = s.sysProxy.Enable(httpAddr, socksAddr, cfg.CustomBypassList)
	}
}

func (s *VpnService) UpdateSettings(cfg settings.AppSettings) error {
	s.mu.Lock()
	err := s.settingsStore.Save(cfg)
	s.mu.Unlock()

	s.updateDohConfiguration()
	s.dohClient.SetSocksPort(cfg.SocksPort)
	_ = winutil.SetAutoStart(cfg.LaunchOnStartup)

	s.notifyListeners()
	return err
}

func (s *VpnService) GetSettings() settings.AppSettings {
	return s.settingsStore.Get()
}

// Background Token Renewal loop (at 50% lifetime)
func (s *VpnService) tokenRenewalLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.RLock()
		pass := s.proxyPass
		isSignedIn := s.fxaClient.HasValidAccessToken()
		s.mu.RUnlock()

		if !isSignedIn || pass == nil {
			continue
		}

		now := time.Now().Unix()
		// If past 50% lifetime (tokens usually last 1 hour)
		remaining := pass.ExpiresAtEpochSeconds - now
		if remaining > 0 && remaining < 1800 {
			logger.GetLogger().Info("Token", "Proxy pass at 50% lifetime, refreshing token in background...")
			accessToken, err := s.fxaClient.EnsureFreshAccessToken()
			if err == nil {
				newPass, err := s.guardianClient.FetchProxyPass(accessToken)
				if err == nil {
					s.mu.Lock()
					s.proxyPass = newPass
					s.mu.Unlock()
					logger.GetLogger().Info("Token", "Transparently refreshed Guardian proxy pass")
					s.notifyListeners()
				}
			}
		}
	}
}

// Watchdog monitoring tunnel health and stalling
func (s *VpnService) watchdogLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.RLock()
		st := s.state
		s.mu.RUnlock()

		if st == StateConnected {
			// Periodic ping to keep active RTT measurement fresh
			pingMs := s.upstreamManager.Ping()
			if pingMs < 0 {
				logger.GetLogger().Warn("Watchdog", "Upstream connection dropped, attempting auto-reconnect...")
				go func() {
					_ = s.Disconnect()
					_ = s.Connect()
				}()
			}
		}
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

	cfg := s.settingsStore.Get()
	var pingVal int64
	if s.upstreamManager != nil {
		pingVal = s.upstreamManager.Ping()
	}

	var quotaLimit, quotaRemaining, quotaReset int64
	if s.proxyPass != nil {
		quotaLimit = s.proxyPass.QuotaLimitBytes
		quotaRemaining = s.proxyPass.QuotaRemainingBytes
		quotaReset = s.proxyPass.QuotaResetEpoch
	}

	return VpnStatus{
		State:                s.state,
		ErrorMessage:         s.errorMessage,
		SelectedCountry:      s.selectedCountry,
		SelectedCity:         s.selectedCity,
		ConnectedNode:        s.connectedNode,
		PingMs:               pingVal,
		UploadSpeedBytes:     s.uploadSpeedBps,
		DownloadSpeedBytes:   s.downloadSpeedBps,
		TotalUploadedBytes:   s.trafficStats.BytesUploaded.Load(),
		TotalDownloadedBytes: s.trafficStats.BytesDownloaded.Load(),
		QuotaLimitBytes:      quotaLimit,
		QuotaRemainingBytes:  quotaRemaining,
		QuotaResetEpoch:      quotaReset,
		IsUnlimited:          s.isUnlimited,
		IsSignedIn:           s.fxaClient.HasValidAccessToken(),
		UserEmail:            s.fxaClient.GetSavedEmail(),
		TlsSplitMode:         cfg.TlsSplitMode,
		SystemProxyEnabled:   s.enableSystemProxy,
		SocksPort:            cfg.SocksPort,
		HttpPort:             cfg.HttpProxyPort,
		SocksBindAddress:     cfg.SocksBindAddress,
		RealIP:               s.realIP,
		RealCountry:          s.realCountry,
		ExitIP:               s.exitIP,
		ExitCountry:          s.exitCountry,
		IsProtected:          s.isProtected,
		ExitCheckEnabled:     cfg.ExitCheckEnabled,
		DnsProvider:          cfg.DnsProvider,
		CustomDohURL:         cfg.CustomDohURL,
		BootstrapDns:         cfg.BootstrapDnsProvider,
		CustomBootstrapIP:    cfg.CustomBootstrapIP,
		CustomEdgeAddress:    cfg.CustomEdgeAddress,
		UpstreamProxyEnabled: cfg.UpstreamProxyEnabled,
		UpstreamProxyType:    cfg.UpstreamProxyType,
		UpstreamProxyHost:    cfg.UpstreamProxyHost,
		UpstreamProxyPort:    cfg.UpstreamProxyPort,
		MinimizeToTray:       cfg.MinimizeToTray,
		CloseToTray:          cfg.CloseToTray,
		LaunchOnStartup:      cfg.LaunchOnStartup,
		BypassLan:            cfg.BypassLan,
		CustomBypassList:     cfg.CustomBypassList,
	}
}

// PingAllServers measures TCP latency to candidate edge nodes for all cities.
func (s *VpnService) PingAllServers() (map[string]int64, error) {
	countries, err := s.serverRepo.FetchCountries()
	if err != nil || len(countries) == 0 {
		countries = remotesettings.BuiltinFallbackCountries()
	}

	var targets []struct {
		Host string
		Port int
		Key  string
	}

	for _, c := range countries {
		for _, city := range c.Cities {
			if len(city.Servers) == 0 {
				continue
			}
			srv := city.Servers[0]
			key := fmt.Sprintf("%s:%s", c.Code, city.Code)
			targets = append(targets, struct {
				Host string
				Port int
				Key  string
			}{
				Host: srv.Hostname,
				Port: 2499,
				Key:  key,
			})
		}
	}

	results := s.pingManager.PingTargets(targets)
	return results, nil
}

func (s *VpnService) PingServer(host string, port int) int64 {
	return s.pingManager.PingServer(host, port)
}

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
