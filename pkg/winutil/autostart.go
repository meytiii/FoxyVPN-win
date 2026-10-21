package winutil

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	RunRegistryKey = `Software\Microsoft\Windows\CurrentVersion\Run`
	AppName        = "FoxyVPN"
)

// SetAutoStart configures whether FoxyVPN launches when Windows logs on.
func SetAutoStart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, RunRegistryKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open Run registry key: %w", err)
	}
	defer k.Close()

	if enable {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		// Pass --minimized to launch in tray on startup
		cmd := fmt.Sprintf("\"%s\" --minimized", exePath)
		return k.SetStringValue(AppName, cmd)
	}

	// Remove startup entry
	_ = k.DeleteValue(AppName)
	return nil
}

// IsAutoStartEnabled checks if FoxyVPN is registered to launch on boot.
func IsAutoStartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, RunRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetStringValue(AppName)
	return err == nil && val != ""
}
