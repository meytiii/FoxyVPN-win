package ping

import (
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	DefaultPingTimeout = 1500 * time.Millisecond
	MaxConcurrentPings = 16
)

type LatencyResult struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Key      string `json:"key"`
	Latency  int64  `json:"latency_ms"`
	IsOnline bool   `json:"is_online"`
}

type PingManager struct {
	mu      sync.RWMutex
	cache   map[string]int64
	timeout time.Duration
}

func NewPingManager(timeout time.Duration) *PingManager {
	if timeout <= 0 {
		timeout = DefaultPingTimeout
	}
	return &PingManager{
		cache:   make(map[string]int64),
		timeout: timeout,
	}
}

// MeasureTcpLatency performs a raw TCP handshake timing to host:port.
func MeasureTcpLatency(host string, port int, timeout time.Duration) int64 {
	if timeout <= 0 {
		timeout = DefaultPingTimeout
	}
	target := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return -1
	}
	defer conn.Close()
	elapsed := time.Since(start).Milliseconds()
	if elapsed <= 0 {
		elapsed = 1
	}
	return elapsed
}

// PingServer measures latency to a single host:port and updates cache.
func (pm *PingManager) PingServer(host string, port int) int64 {
	key := fmt.Sprintf("%s:%d", host, port)
	latency := MeasureTcpLatency(host, port, pm.timeout)
	pm.mu.Lock()
	pm.cache[key] = latency
	pm.mu.Unlock()
	return latency
}

// PingTargets batches concurrent pings across multiple endpoints.
func (pm *PingManager) PingTargets(targets []struct{ Host string; Port int; Key string }) map[string]int64 {
	results := make(map[string]int64)
	if len(targets) == 0 {
		return results
	}

	sem := make(chan struct{}, MaxConcurrentPings)
	var wg sync.WaitGroup
	var resMu sync.Mutex

	for _, t := range targets {
		wg.Add(1)
		go func(host string, port int, key string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			latency := MeasureTcpLatency(host, port, pm.timeout)
			resMu.Lock()
			results[key] = latency
			resMu.Unlock()

			pm.mu.Lock()
			pm.cache[key] = latency
			pm.mu.Unlock()
		}(t.Host, t.Port, t.Key)
	}

	wg.Wait()
	return results
}

// GetCached returns currently cached latency for a key.
func (pm *PingManager) GetCached(key string) (int64, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	val, ok := pm.cache[key]
	return val, ok
}

// GetAllCached returns a copy of all cached latencies.
func (pm *PingManager) GetAllCached() map[string]int64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	copied := make(map[string]int64, len(pm.cache))
	for k, v := range pm.cache {
		copied[k] = v
	}
	return copied
}
