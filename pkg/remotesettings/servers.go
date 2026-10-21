package remotesettings

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	RemoteSettingsURL   = "https://firefox.settings.services.mozilla.com/v1/buckets/main/collections/vpn-serverlist/records"
	MozillaVpnUserAgent = "MozillaVPN/2.35.0 (sys:windows; iap:true)"
)

type VpnProtocol struct {
	Name           string `json:"name"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Scheme         string `json:"scheme"`
	TemplateString string `json:"templateString"`
}

type VpnServerNode struct {
	Hostname    string        `json:"hostname"`
	Port        int           `json:"port"`
	Quarantined bool          `json:"quarantined"`
	Protocols   []VpnProtocol `json:"protocols"`
}

type VpnCity struct {
	Name    string          `json:"name"`
	Code    string          `json:"code"`
	Servers []VpnServerNode `json:"servers"`
}

type VpnCountry struct {
	Name   string    `json:"name"`
	Code   string    `json:"code"`
	Cities []VpnCity `json:"cities"`
}

type ProxyCandidate struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	CountryCode string `json:"country_code"`
	CountryName string `json:"country_name"`
	CityName    string `json:"city_name"`
	CityCode    string `json:"city_code"`
	PingMs      int64  `json:"ping_ms"`
}

type ServerRepository struct {
	mu         sync.RWMutex
	cachePath  string
	httpClient *http.Client
	countries  []VpnCountry
}

func NewServerRepository(cacheDir string) *ServerRepository {
	_ = os.MkdirAll(cacheDir, 0700)
	return &ServerRepository{
		cachePath: filepath.Join(cacheDir, "vpn_serverlist_cache.json"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// FetchCountries retrieves the latest server list from Remote Settings or falls back to cache/builtins.
func (r *ServerRepository) FetchCountries() ([]VpnCountry, error) {
	req, err := http.NewRequest(http.MethodGet, RemoteSettingsURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", MozillaVpnUserAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := r.httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			bodyBytes, readErr := io.ReadAll(resp.Body)
			if readErr == nil && len(bodyBytes) > 0 {
				countries := parseRecords(bodyBytes)
				if len(countries) > 0 {
					_ = os.WriteFile(r.cachePath, bodyBytes, 0644)
					r.mu.Lock()
					r.countries = countries
					r.mu.Unlock()
					return countries, nil
				}
			}
		}
	}

	cached := r.loadCache()
	if len(cached) > 0 {
		r.mu.Lock()
		r.countries = cached
		r.mu.Unlock()
		return cached, nil
	}

	fallback := BuiltinFallbackCountries()
	r.mu.Lock()
	r.countries = fallback
	r.mu.Unlock()
	return fallback, nil
}

func (r *ServerRepository) loadCache() []VpnCountry {
	data, err := os.ReadFile(r.cachePath)
	if err != nil {
		return nil
	}
	return parseRecords(data)
}

func parseRecords(data []byte) []VpnCountry {
	var root struct {
		Data []struct {
			Country struct {
				Name   string `json:"name"`
				Code   string `json:"code"`
				Cities []struct {
					Name    string `json:"name"`
					Code    string `json:"code"`
					Servers []struct {
						Hostname    string `json:"hostname"`
						Port        int    `json:"port"`
						Quarantined bool   `json:"quarantined"`
						Protocols   []struct {
							Name   string `json:"name"`
							Host   string `json:"host"`
							Port   int    `json:"port"`
							Scheme string `json:"scheme"`
						} `json:"protocols"`
					} `json:"servers"`
				} `json:"cities"`
			} `json:"country"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}

	var countries []VpnCountry
	for _, rec := range root.Data {
		c := rec.Country
		if strings.EqualFold(c.Name, "CatchAll Anycast") || c.Code == "" || len(c.Cities) == 0 {
			continue
		}
		var cities []VpnCity
		for _, cityRec := range c.Cities {
			var servers []VpnServerNode
			for _, srv := range cityRec.Servers {
				var protos []VpnProtocol
				for _, proto := range srv.Protocols {
					protos = append(protos, VpnProtocol{
						Name:   proto.Name,
						Host:   proto.Host,
						Port:   proto.Port,
						Scheme: proto.Scheme,
					})
				}
				servers = append(servers, VpnServerNode{
					Hostname:    srv.Hostname,
					Port:        srv.Port,
					Quarantined: srv.Quarantined,
					Protocols:   protos,
				})
			}
			cities = append(cities, VpnCity{
				Name:    cityRec.Name,
				Code:    cityRec.Code,
				Servers: servers,
			})
		}
		countries = append(countries, VpnCountry{
			Name:   c.Name,
			Code:   c.Code,
			Cities: cities,
		})
	}
	return countries
}

// CandidatesForCountry generates a list of operational proxy candidates for a given country code.
func CandidatesForCountry(countries []VpnCountry, countryCode string) []ProxyCandidate {
	var candidates []ProxyCandidate
	for _, c := range countries {
		if !strings.EqualFold(c.Code, countryCode) && countryCode != "" && countryCode != "AUTO" {
			continue
		}
		for _, city := range c.Cities {
			for _, srv := range city.Servers {
				if srv.Quarantined {
					continue
				}
				host, port := resolveServerTarget(srv)
				if host == "" || port <= 0 {
					continue
				}
				candidates = append(candidates, ProxyCandidate{
					Host:        host,
					Port:        port,
					CountryCode: c.Code,
					CountryName: c.Name,
					CityName:    city.Name,
					CityCode:    city.Code,
				})
			}
		}
	}
	return candidates
}

func resolveServerTarget(server VpnServerNode) (string, int) {
	for _, proto := range server.Protocols {
		if strings.EqualFold(proto.Name, "connect") {
			host := proto.Host
			if host == "" {
				host = server.Hostname
			}
			port := proto.Port
			if port == 0 {
				port = server.Port
			}
			return host, port
		}
	}
	return server.Hostname, server.Port
}

// BuiltinFallbackCountries returns hardcoded Fastly edge nodes for Mozilla VPN.
func BuiltinFallbackCountries() []VpnCountry {
	return []VpnCountry{
		{
			Name: "United States",
			Code: "US",
			Cities: []VpnCity{
				{
					Name: "New York",
					Code: "nyc",
					Servers: []VpnServerNode{
						{
							Hostname: "us-nyc.secureproxy.mozilla.com",
							Port:     443,
							Protocols: []VpnProtocol{
								{Name: "connect", Host: "us-nyc.secureproxy.mozilla.com", Port: 443},
							},
						},
					},
				},
				{
					Name: "San Jose",
					Code: "sjc",
					Servers: []VpnServerNode{
						{
							Hostname: "us-sjc.secureproxy.mozilla.com",
							Port:     443,
							Protocols: []VpnProtocol{
								{Name: "connect", Host: "us-sjc.secureproxy.mozilla.com", Port: 443},
							},
						},
					},
				},
			},
		},
		{
			Name: "Germany",
			Code: "DE",
			Cities: []VpnCity{
				{
					Name: "Frankfurt",
					Code: "fra",
					Servers: []VpnServerNode{
						{
							Hostname: "de-fra.secureproxy.mozilla.com",
							Port:     443,
							Protocols: []VpnProtocol{
								{Name: "connect", Host: "de-fra.secureproxy.mozilla.com", Port: 443},
							},
						},
					},
				},
			},
		},
		{
			Name: "Netherlands",
			Code: "NL",
			Cities: []VpnCity{
				{
					Name: "Amsterdam",
					Code: "ams",
					Servers: []VpnServerNode{
						{
							Hostname: "nl-ams.secureproxy.mozilla.com",
							Port:     443,
							Protocols: []VpnProtocol{
								{Name: "connect", Host: "nl-ams.secureproxy.mozilla.com", Port: 443},
							},
						},
					},
				},
			},
		},
		{
			Name: "United Kingdom",
			Code: "GB",
			Cities: []VpnCity{
				{
					Name: "London",
					Code: "lon",
					Servers: []VpnServerNode{
						{
							Hostname: "gb-lon.secureproxy.mozilla.com",
							Port:     443,
							Protocols: []VpnProtocol{
								{Name: "connect", Host: "gb-lon.secureproxy.mozilla.com", Port: 443},
							},
						},
					},
				},
			},
		},
		{
			Name: "Canada",
			Code: "CA",
			Cities: []VpnCity{
				{
					Name: "Toronto",
					Code: "yyz",
					Servers: []VpnServerNode{
						{
							Hostname: "ca-yyz.secureproxy.mozilla.com",
							Port:     443,
							Protocols: []VpnProtocol{
								{Name: "connect", Host: "ca-yyz.secureproxy.mozilla.com", Port: 443},
							},
						},
					},
				},
			},
		},
	}
}
