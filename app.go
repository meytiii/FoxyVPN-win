package main

import (
	"context"
	"fmt"

	"foxyvpn/pkg/core"
	"foxyvpn/pkg/remotesettings"
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
		_, _ = a.service.RestoreSession()
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
