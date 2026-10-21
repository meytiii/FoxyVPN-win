package tun

import (
	_ "embed"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/xjasonlyu/tun2socks/v2/engine"
	"golang.org/x/sys/windows"

	"foxyvpn/pkg/logger"
)

//go:embed wintun.dll
var embeddedWintunDLL []byte

const (
	AdapterName = "FoxyVPN"
	TunnelIP    = "10.0.85.2"
	TunnelMask  = "255.255.255.0"
	TunnelGW    = "10.0.85.1"
	TunnelDNS   = "1.1.1.1"
)

type TunnelManager struct {
	mu           sync.Mutex
	isRunning    bool
	edgeHost     string
	originalGW   string
	originalNIC  string
}

func NewTunnelManager() *TunnelManager {
	return &TunnelManager{}
}

// IsElevated checks whether the current process has Windows Administrator rights.
func IsElevated() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	return err == nil && member
}

// EnsureWintunDLL makes sure wintun.dll exists in the application directory or working directory.
func EnsureWintunDLL() error {
	exePath, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exePath)
		target := filepath.Join(dir, "wintun.dll")
		if _, err := os.Stat(target); os.IsNotExist(err) && len(embeddedWintunDLL) > 0 {
			if writeErr := os.WriteFile(target, embeddedWintunDLL, 0755); writeErr == nil {
				return nil
			}
		}
	}

	if _, err := os.Stat("wintun.dll"); os.IsNotExist(err) && len(embeddedWintunDLL) > 0 {
		return os.WriteFile("wintun.dll", embeddedWintunDLL, 0755)
	}
	return nil
}

// Start initiates the Wintun adapter and configures full system IP routing through tun2socks.
func (m *TunnelManager) Start(socksPort int, connectedNode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return nil
	}

	if !IsElevated() {
		return fmt.Errorf("Administrator privileges required to create Windows TUN adapter")
	}

	if err := EnsureWintunDLL(); err != nil {
		logger.GetLogger().Warn("TUN", fmt.Sprintf("Could not verify wintun.dll: %v", err))
	}

	// 1. Detect default gateway and interface
	gw, nic, err := getDefaultGateway()
	if err != nil {
		logger.GetLogger().Warn("TUN", fmt.Sprintf("Could not detect default gateway: %v", err))
	} else {
		m.originalGW = gw
		m.originalNIC = nic
		logger.GetLogger().Info("TUN", fmt.Sprintf("Detected default gateway: %s on interface %s", gw, nic))
	}

	// Extract remote edge IP/host
	edgeHost := connectedNode
	if strings.Contains(edgeHost, ":") {
		h, _, err := net.SplitHostPort(edgeHost)
		if err == nil {
			edgeHost = h
		}
	}
	m.edgeHost = edgeHost

	// 2. Start tun2socks engine
	key := &engine.Key{
		Device:   fmt.Sprintf("tun://%s", AdapterName),
		Proxy:    fmt.Sprintf("socks5://127.0.0.1:%d", socksPort),
		MTU:      1500,
		LogLevel: "silent",
	}

	engine.Insert(key)
	engine.Start()
	time.Sleep(400 * time.Millisecond)

	// 3. Configure IP and DNS on the TUN adapter
	// netsh interface ip set address name="FoxyVPN" static 10.0.85.2 255.255.255.0 10.0.85.1 1
	cmdSetIP := exec.Command("netsh", "interface", "ip", "set", "address",
		fmt.Sprintf("name=\"%s\"", AdapterName), "static", TunnelIP, TunnelMask, TunnelGW, "1")
	cmdSetIP.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmdSetIP.CombinedOutput(); err != nil {
		logger.GetLogger().Warn("TUN", fmt.Sprintf("netsh set address warning: %v, out: %s", err, string(out)))
	}

	cmdSetDNS := exec.Command("netsh", "interface", "ip", "set", "dns",
		fmt.Sprintf("name=\"%s\"", AdapterName), "static", TunnelDNS)
	cmdSetDNS.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmdSetDNS.Run()

	// 4. Configure routes
	// Route Fastly edge directly via physical gateway to prevent tunnel loop
	if m.originalGW != "" && m.edgeHost != "" {
		ips, err := net.LookupIP(m.edgeHost)
		if err == nil && len(ips) > 0 {
			for _, ip := range ips {
				if ip4 := ip.To4(); ip4 != nil {
					cmdEdge := exec.Command("route", "add", ip4.String(), "mask", "255.255.255.255", m.originalGW, "metric", "1")
					cmdEdge.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
					_ = cmdEdge.Run()
				}
			}
		}
	}

	// Add 0.0.0.0/1 and 128.0.0.0/1 through TUN adapter (overrides default route safely)
	cmdR1 := exec.Command("route", "add", "0.0.0.0", "mask", "128.0.0.0", TunnelGW, "metric", "5")
	cmdR1.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmdR1.Run()

	cmdR2 := exec.Command("route", "add", "128.0.0.0", "mask", "128.0.0.0", TunnelGW, "metric", "5")
	cmdR2.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmdR2.Run()

	m.isRunning = true
	logger.GetLogger().Info("TUN", "FoxyVPN Tunnel Mode (Wintun) successfully started")
	return nil
}

// Stop terminates the TUN adapter, removes routing rules, and stops the tun2socks engine.
func (m *TunnelManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning {
		return nil
	}

	// Remove tunnel routes
	cmdR1 := exec.Command("route", "delete", "0.0.0.0", "mask", "128.0.0.0", TunnelGW)
	cmdR1.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmdR1.Run()

	cmdR2 := exec.Command("route", "delete", "128.0.0.0", "mask", "128.0.0.0", TunnelGW)
	cmdR2.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmdR2.Run()

	if m.edgeHost != "" {
		ips, err := net.LookupIP(m.edgeHost)
		if err == nil {
			for _, ip := range ips {
				if ip4 := ip.To4(); ip4 != nil {
					cmdEdge := exec.Command("route", "delete", ip4.String())
					cmdEdge.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
					_ = cmdEdge.Run()
				}
			}
		}
	}

	// Stop engine
	engine.Stop()
	m.isRunning = false
	logger.GetLogger().Info("TUN", "FoxyVPN Tunnel Mode stopped")
	return nil
}

func (m *TunnelManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isRunning
}

// getDefaultGateway queries the active default gateway IP from Windows route table.
func getDefaultGateway() (string, string, error) {
	cmd := exec.Command("route", "print", "0.0.0.0")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 5 && fields[0] == "0.0.0.0" && fields[1] == "0.0.0.0" {
			gateway := fields[2]
			nic := fields[3]
			return gateway, nic, nil
		}
	}
	return "", "", fmt.Errorf("no default gateway found in route print")
}
