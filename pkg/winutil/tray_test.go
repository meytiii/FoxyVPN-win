package winutil

import (
	"testing"
)

func TestLoadAppTrayIcon(t *testing.T) {
	h := loadAppTrayIcon()
	if h == 0 {
		t.Fatalf("expected non-zero HICON for app tray icon, got 0")
	}
	t.Logf("Successfully loaded tray icon handle: 0x%x", h)
}
