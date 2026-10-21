package winproxy

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	InternetSettingsKey             = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	INTERNET_OPTION_SETTINGS_CHANGED = 39
	INTERNET_OPTION_REFRESH          = 37
)

var (
	wininet                = windows.NewLazySystemDLL("wininet.dll")
	procInternetSetOptionW = wininet.NewProc("InternetSetOptionW")
)

// SystemProxyManager manages the Windows WinINet system-wide proxy settings.
type SystemProxyManager struct {
	mu           sync.Mutex
	wasEnabled   bool
	origEnabled  uint64
	origServer   string
	origOverride string
	hasBackup    bool
}

// NewSystemProxyManager creates a proxy manager instance.
func NewSystemProxyManager() *SystemProxyManager {
	return &SystemProxyManager{}
}

// Enable configures Windows to route web traffic through the local proxy servers.
func (m *SystemProxyManager) Enable(httpProxyAddr, socksProxyAddr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	k, err := registry.OpenKey(registry.CURRENT_USER, InternetSettingsKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open Internet Settings registry key: %w", err)
	}
	defer k.Close()

	if !m.hasBackup {
		origEn, _, err := k.GetIntegerValue("ProxyEnable")
		if err == nil {
			m.origEnabled = origEn
		}
		origSrv, _, err := k.GetStringValue("ProxyServer")
		if err == nil {
			m.origServer = origSrv
		}
		origOvr, _, err := k.GetStringValue("ProxyOverride")
		if err == nil {
			m.origOverride = origOvr
		}
		m.hasBackup = true
	}

	proxyServerVal := fmt.Sprintf("http=%s;https=%s;socks=%s", httpProxyAddr, httpProxyAddr, socksProxyAddr)
	proxyOverrideVal := "<-loopback>;<local>;localhost;127.*;10.*;192.168.*"

	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("failed to set ProxyEnable: %w", err)
	}
	if err := k.SetStringValue("ProxyServer", proxyServerVal); err != nil {
		return fmt.Errorf("failed to set ProxyServer: %w", err)
	}
	if err := k.SetStringValue("ProxyOverride", proxyOverrideVal); err != nil {
		return fmt.Errorf("failed to set ProxyOverride: %w", err)
	}

	m.wasEnabled = true
	m.refreshSettings()
	return nil
}

// Disable restores prior Windows proxy settings or unsets the proxy.
func (m *SystemProxyManager) Disable() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.wasEnabled && !m.hasBackup {
		return nil
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, InternetSettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer k.Close()

	if m.hasBackup {
		_ = k.SetDWordValue("ProxyEnable", uint32(m.origEnabled))
		if m.origServer != "" {
			_ = k.SetStringValue("ProxyServer", m.origServer)
		} else {
			_ = k.DeleteValue("ProxyServer")
		}
		if m.origOverride != "" {
			_ = k.SetStringValue("ProxyOverride", m.origOverride)
		}
	} else {
		_ = k.SetDWordValue("ProxyEnable", 0)
	}

	m.wasEnabled = false
	m.refreshSettings()
	return nil
}

func (m *SystemProxyManager) refreshSettings() {
	_, _, _ = procInternetSetOptionW.Call(0, INTERNET_OPTION_SETTINGS_CHANGED, 0, 0)
	_, _, _ = procInternetSetOptionW.Call(0, INTERNET_OPTION_REFRESH, 0, 0)
}

// IsSystemProxyActive checks if Windows currently has ProxyEnable = 1.
func IsSystemProxyActive() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, InternetSettingsKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetIntegerValue("ProxyEnable")
	return err == nil && val == 1
}

// EnsureWininetOption forces a flush on process exit if needed.
func EnsureWininetOption() {
	_, _, _ = procInternetSetOptionW.Call(0, uintptr(INTERNET_OPTION_SETTINGS_CHANGED), 0, uintptr(unsafe.Pointer(nil)))
	_, _, _ = procInternetSetOptionW.Call(0, uintptr(INTERNET_OPTION_REFRESH), 0, uintptr(unsafe.Pointer(nil)))
}
