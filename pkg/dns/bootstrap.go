package dns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type BootstrapProviderInfo struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Addresses   []string `json:"addresses"`
}

var StandardBootstrapProviders = []BootstrapProviderInfo{
	{
		ID:          "AUTOMATIC",
		Label:       "Automatic (Fastest Race)",
		Description: "Concurrently races 9.9.9.9, 1.1.1.1, and 8.8.8.8",
		Addresses:   []string{"9.9.9.9", "1.1.1.1", "8.8.8.8"},
	},
	{
		ID:          "CLOUDFLARE",
		Label:       "Cloudflare",
		Description: "1.1.1.1, 1.0.0.1",
		Addresses:   []string{"1.1.1.1", "1.0.0.1"},
	},
	{
		ID:          "GOOGLE",
		Label:       "Google",
		Description: "8.8.8.8, 8.8.4.4",
		Addresses:   []string{"8.8.8.8", "8.8.4.4"},
	},
	{
		ID:          "QUAD9",
		Label:       "Quad9",
		Description: "9.9.9.9, 149.112.112.112",
		Addresses:   []string{"9.9.9.9", "149.112.112.112"},
	},
	{
		ID:          "SYSTEM",
		Label:       "System DNS",
		Description: "Default Windows network resolver",
		Addresses:   nil,
	},
	{
		ID:          "CUSTOM",
		Label:       "Custom Resolver IP",
		Description: "User-specified DNS resolver address",
		Addresses:   nil,
	},
}

type BootstrapResolver struct {
	mu           sync.RWMutex
	cache        map[string]string
	cacheExpires map[string]time.Time
}

func NewBootstrapResolver() *BootstrapResolver {
	return &BootstrapResolver{
		cache:        make(map[string]string),
		cacheExpires: make(map[string]time.Time),
	}
}

// ResolveEdgeHost resolves hostname using either system resolver or racing bootstrap endpoints.
func (r *BootstrapResolver) ResolveEdgeHost(ctx context.Context, hostname string, bootstrapAddresses []string) (string, error) {
	host := strings.TrimSpace(strings.ToLower(hostname))
	if host == "" {
		return "", fmt.Errorf("empty hostname")
	}

	// Direct IP literal
	if net.ParseIP(host) != nil {
		return host, nil
	}

	// Check cache
	r.mu.RLock()
	cachedIP, exists := r.cache[host]
	expires := r.cacheExpires[host]
	r.mu.RUnlock()
	if exists && time.Now().Before(expires) {
		return cachedIP, nil
	}

	if len(bootstrapAddresses) == 0 {
		// Use system DNS
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil || len(ips) == 0 {
			return "", fmt.Errorf("system resolver lookup failed for %s: %w", host, err)
		}
		selected := ips[0].String()
		r.storeCache(host, selected, 5*time.Minute)
		return selected, nil
	}

	// Race bootstrap DNS addresses
	type raceResult struct {
		ip  string
		err error
	}
	resultChan := make(chan raceResult, len(bootstrapAddresses))
	raceCtx, raceCancel := context.WithTimeout(ctx, 4*time.Second)
	defer raceCancel()

	for _, addr := range bootstrapAddresses {
		go func(resolverIP string) {
			resolver := &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					d := net.Dialer{Timeout: 2 * time.Second}
					return d.DialContext(ctx, "udp", net.JoinHostPort(resolverIP, "53"))
				},
			}
			ips, err := resolver.LookupIP(raceCtx, "ip", host)
			if err != nil || len(ips) == 0 {
				resultChan <- raceResult{err: fmt.Errorf("%s failed: %w", resolverIP, err)}
				return
			}
			resultChan <- raceResult{ip: ips[0].String()}
		}(addr)
	}

	var lastErr error
	for i := 0; i < len(bootstrapAddresses); i++ {
		res := <-resultChan
		if res.ip != "" {
			raceCancel()
			r.storeCache(host, res.ip, 5*time.Minute)
			return res.ip, nil
		}
		lastErr = res.err
	}

	// Fallback to system resolver if all bootstrap IPs failed
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err == nil && len(ips) > 0 {
		selected := ips[0].String()
		r.storeCache(host, selected, 5*time.Minute)
		return selected, nil
	}

	return "", fmt.Errorf("bootstrap resolution failed for %s: %v", host, lastErr)
}

func (r *BootstrapResolver) storeCache(host, ip string, ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[host] = ip
	r.cacheExpires[host] = time.Now().Add(ttl)
}

func (r *BootstrapResolver) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = make(map[string]string)
	r.cacheExpires = make(map[string]time.Time)
}
