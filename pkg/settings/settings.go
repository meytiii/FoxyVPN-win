package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type AppSettings struct {
	ExitCheckEnabled      bool   `json:"exit_check_enabled"`
	DnsProvider           string `json:"dns_provider"`
	CustomDohURL          string `json:"custom_doh_url"`
	BootstrapDnsProvider  string `json:"bootstrap_dns_provider"`
	CustomBootstrapIP     string `json:"custom_bootstrap_ip"`
	SocksPort             int    `json:"socks_port"`
	SocksBindAddress      string `json:"socks_bind_address"`
	HttpProxyEnabled      bool   `json:"http_proxy_enabled"`
	HttpProxyPort         int    `json:"http_proxy_port"`
	ProxyOnlyMode         bool   `json:"proxy_only_mode"`
	CustomEdgeAddress     string `json:"custom_edge_address"`
	TlsSplitMode          int    `json:"tls_split_mode"`
	UpstreamProxyEnabled  bool   `json:"upstream_proxy_enabled"`
	UpstreamProxyType     string `json:"upstream_proxy_type"`
	UpstreamProxyHost     string `json:"upstream_proxy_host"`
	UpstreamProxyPort     int    `json:"upstream_proxy_port"`
	UpstreamProxyUsername string `json:"upstream_proxy_username"`
	UpstreamProxyPassword string `json:"upstream_proxy_password"`
	MinimizeToTray        bool   `json:"minimize_to_tray"`
	CloseToTray           bool   `json:"close_to_tray"`
	LaunchOnStartup       bool   `json:"launch_on_startup"`
	BypassLan             bool   `json:"bypass_lan"`
	CustomBypassList      string `json:"custom_bypass_list"`
}

func DefaultSettings() AppSettings {
	return AppSettings{
		ExitCheckEnabled:      true,
		DnsProvider:           "SYSTEM_DEFAULT",
		CustomDohURL:          "",
		BootstrapDnsProvider:  "AUTOMATIC",
		CustomBootstrapIP:     "",
		SocksPort:             1080,
		SocksBindAddress:      "127.0.0.1",
		HttpProxyEnabled:      true,
		HttpProxyPort:         8080,
		ProxyOnlyMode:         false,
		CustomEdgeAddress:     "",
		TlsSplitMode:          1, // FirstByte default
		UpstreamProxyEnabled:  false,
		UpstreamProxyType:     "SOCKS5",
		UpstreamProxyHost:     "",
		UpstreamProxyPort:     1080,
		UpstreamProxyUsername: "",
		UpstreamProxyPassword: "",
		MinimizeToTray:        true,
		CloseToTray:           true,
		LaunchOnStartup:       false,
		BypassLan:             true,
		CustomBypassList:      "<local>;192.168.0.0/16;10.0.0.0/8;172.16.0.0/12;127.0.0.1;localhost",
	}
}

type SettingsStore struct {
	mu       sync.RWMutex
	filePath string
	current  AppSettings
}

func NewSettingsStore(appDir string) *SettingsStore {
	_ = os.MkdirAll(appDir, 0700)
	filePath := filepath.Join(appDir, "settings.json")
	store := &SettingsStore{
		filePath: filePath,
		current:  DefaultSettings(),
	}
	store.load()
	return store
}

func (s *SettingsStore) Get() AppSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *SettingsStore) Save(settings AppSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = settings

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *SettingsStore) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}
	var loaded AppSettings
	if err := json.Unmarshal(data, &loaded); err == nil {
		if loaded.SocksPort <= 0 {
			loaded.SocksPort = 1080
		}
		if loaded.HttpProxyPort <= 0 {
			loaded.HttpProxyPort = 8080
		}
		if loaded.SocksBindAddress == "" {
			loaded.SocksBindAddress = "127.0.0.1"
		}
		s.current = loaded
	}
}
