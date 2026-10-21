package main

import (
	"context"
	"fmt"
	"time"

	"foxyvpn/pkg/core"
	"foxyvpn/pkg/exitcheck"
	"foxyvpn/pkg/logger"
	"foxyvpn/pkg/remotesettings"
	"foxyvpn/pkg/settings"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct manages the desktop application lifecycle and Go-Frontend bridge.
type App struct {
	ctx     context.Context
	service *core.VpnService
}

// NewApp creates a new App application instance.
func NewApp() *App {
	svc, err := core.NewVpnService()
	if err != nil {
		fmt.Printf("Error initializing VpnService: %v\n", err)
	}
	return &App{
		service: svc,
	}
}

// startup is called when the Wails application starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.service != nil {
		a.service.RegisterListener(func(status core.VpnStatus) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "vpn:status", status)
			}
		})
		logger.GetLogger().RegisterListener(func(entry logger.LogEntry) {
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "vpn:log", entry)
			}
		})
		go func() {
			_, _ = a.service.RestoreSession()
		}()
	}
}

// shutdown is called before the application terminates.
func (a *App) shutdown(ctx context.Context) {
	if a.service != nil {
		_ = a.service.Disconnect()
	}
}

// GetStatus returns the current live VPN state and telemetry.
func (a *App) GetStatus() core.VpnStatus {
	if a.service == nil {
		return core.VpnStatus{State: core.StateError, ErrorMessage: "service uninitialized"}
	}
	return a.service.GetStatus()
}

// Connect starts the VPN tunnel.
func (a *App) Connect() error {
	if a.service == nil {
		return fmt.Errorf("service uninitialized")
	}
	return a.service.Connect()
}

// Disconnect stops the VPN tunnel.
func (a *App) Disconnect() error {
	if a.service == nil {
		return fmt.Errorf("service uninitialized")
	}
	return a.service.Disconnect()
}

// SetSelectedCountry updates target location.
func (a *App) SetSelectedCountry(countryCode, cityCode string) {
	if a.service != nil {
		a.service.SetSelectedCountry(countryCode, cityCode)
	}
}

// SetTlsSplitMode toggles TLS ClientHello packet splitting.
func (a *App) SetTlsSplitMode(mode int) {
	if a.service != nil {
		a.service.SetTlsSplitMode(mode)
	}
}

// SetSystemProxyEnabled toggles Windows WinINet system routing.
func (a *App) SetSystemProxyEnabled(enable bool) {
	if a.service != nil {
		a.service.SetSystemProxyEnabled(enable)
	}
}

// GetSettings retrieves current application preferences.
func (a *App) GetSettings() settings.AppSettings {
	if a.service == nil {
		return settings.DefaultSettings()
	}
	return a.service.GetSettings()
}

// UpdateSettings persists updated application preferences.
func (a *App) UpdateSettings(cfg settings.AppSettings) error {
	if a.service == nil {
		return fmt.Errorf("service uninitialized")
	}
	return a.service.UpdateSettings(cfg)
}

// PingServer measures latency to a specific edge node.
func (a *App) PingServer(host string, port int) int64 {
	if a.service == nil {
		return -1
	}
	return a.service.PingServer(host, port)
}

// PingAllServers measures latency to all available locations in parallel.
func (a *App) PingAllServers() (map[string]int64, error) {
	if a.service == nil {
		return nil, fmt.Errorf("service uninitialized")
	}
	return a.service.PingAllServers()
}

// GetLogs returns buffered log entries filtered by level (ALL, INFO, PROBLEMS).
func (a *App) GetLogs(filter string) []logger.LogEntry {
	return logger.GetLogger().GetEntries(filter)
}

// ClearLogs clears the in-memory log buffer.
func (a *App) ClearLogs() {
	logger.GetLogger().Clear()
}

// ExportLogs writes log buffer to a text file and returns path.
func (a *App) ExportLogs() (string, error) {
	return logger.GetLogger().SaveToFile("")
}

// ExportLogsText returns formatted logs as string for clipboard copy.
func (a *App) ExportLogsText() string {
	return logger.GetLogger().ExportAsText()
}

// FxaLogin starts authentication with Firefox Accounts.
func (a *App) FxaLogin(email, password string) (bool, error) {
	if a.service == nil {
		return false, fmt.Errorf("service uninitialized")
	}
	return a.service.FxaLogin(email, password)
}

// FxaSubmit2FACode completes authentication with 2FA verification code.
func (a *App) FxaSubmit2FACode(code string) error {
	if a.service == nil {
		return fmt.Errorf("service uninitialized")
	}
	return a.service.FxaSubmit2FACode(code)
}

// FxaSignOut logs out and deletes local session tokens.
func (a *App) FxaSignOut() error {
	if a.service == nil {
		return fmt.Errorf("service uninitialized")
	}
	return a.service.FxaSignOut()
}

// GetCountries fetches the list of available VPN exit locations.
func (a *App) GetCountries() ([]remotesettings.VpnCountry, error) {
	if a.service == nil {
		return nil, fmt.Errorf("service uninitialized")
	}
	return a.service.GetCountries()
}

// CheckDirectIp returns public IP information directly.
func (a *App) CheckDirectIp() (*exitcheck.IpLocationInfo, error) {
	checker := exitcheck.NewExitChecker()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return checker.FetchDirectIpInfo(ctx)
}

// CheckProxiedIp returns public IP information through the local SOCKS5 proxy.
func (a *App) CheckProxiedIp(socksPort int) (*exitcheck.IpLocationInfo, error) {
	checker := exitcheck.NewExitChecker()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return checker.FetchProxiedIpInfo(ctx, socksPort)
}
