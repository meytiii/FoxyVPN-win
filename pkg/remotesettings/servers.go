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
	var rawRoot struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &rawRoot); err != nil {
		return nil
	}

	type protoJSON struct {
		Name   string `json:"name"`
		Host   string `json:"host"`
		Port   int    `json:"port"`
		Scheme string `json:"scheme"`
	}

	type serverJSON struct {
		Hostname    string      `json:"hostname"`
		Port        int         `json:"port"`
		Quarantined bool        `json:"quarantined"`
		Protocols   []protoJSON `json:"protocols"`
	}

	type cityJSON struct {
		Name    string       `json:"name"`
		Code    string       `json:"code"`
		Servers []serverJSON `json:"servers"`
	}

	type itemJSON struct {
		Name    string     `json:"name"`
		Code    string     `json:"code"`
		Cities  []cityJSON `json:"cities"`
		Country *struct {
			Name   string     `json:"name"`
			Code   string     `json:"code"`
			Cities []cityJSON `json:"cities"`
		} `json:"country"`
	}

	var countries []VpnCountry
	for _, recRaw := range rawRoot.Data {
		var item itemJSON
		if err := json.Unmarshal(recRaw, &item); err != nil {
			continue
		}

		name := item.Name
		code := item.Code
		citiesList := item.Cities

		if item.Country != nil && item.Country.Code != "" {
			name = item.Country.Name
			code = item.Country.Code
			if len(item.Country.Cities) > 0 {
				citiesList = item.Country.Cities
			}
		}

		if strings.EqualFold(name, "CatchAll Anycast") || code == "" || len(citiesList) == 0 {
			continue
		}

		var cities []VpnCity
		for _, cityRec := range citiesList {
			var servers []VpnServerNode
			for _, srv := range cityRec.Servers {
				var protos []VpnProtocol
				for _, p := range srv.Protocols {
					protos = append(protos, VpnProtocol{
						Name:   p.Name,
						Host:   p.Host,
						Port:   p.Port,
						Scheme: p.Scheme,
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
			Name:   name,
			Code:   code,
			Cities: cities,
		})
	}
	return countries
}

// CandidatesForCountry generates a list of operational proxy candidates for a given country code.
func CandidatesForCountry(countries []VpnCountry, countryCode string) []ProxyCandidate {
	var candidates []ProxyCandidate
	for _, c := range countries {
		if !strings.EqualFold(c.Code, countryCode) && countryCode != "" && countryCode != "AUTO" && countryCode != "REC" {
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
	port := server.Port
	if port <= 0 {
		port = 2499
	}
	return server.Hostname, port
}

// BuiltinFallbackCountries returns standard Fastly edge nodes for Mozilla VPN.
func BuiltinFallbackCountries() []VpnCountry {
	return []VpnCountry{
		{
			Name: "Recommended (Fastest)",
			Code: "REC",
			Cities: []VpnCity{
				{
					Name: "Anycast Edge",
					Code: "REC",
					Servers: []VpnServerNode{
						{Hostname: "p.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "United States",
			Code: "US",
			Cities: []VpnCity{
				{
					Name: "United States",
					Code: "US",
					Servers: []VpnServerNode{
						{Hostname: "us.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "Germany",
			Code: "DE",
			Cities: []VpnCity{
				{
					Name: "Munich",
					Code: "MUC",
					Servers: []VpnServerNode{
						{Hostname: "muc139.m1.fastly-masque.net", Port: 2499},
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
					Code: "EGLC",
					Servers: []VpnServerNode{
						{Hostname: "eglc860.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "Netherlands",
			Code: "NL",
			Cities: []VpnCity{
				{
					Name: "Rotterdam",
					Code: "EHRD",
					Servers: []VpnServerNode{
						{Hostname: "ehrd229.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "France",
			Code: "FR",
			Cities: []VpnCity{
				{
					Name: "Paris",
					Code: "LFPB",
					Servers: []VpnServerNode{
						{Hostname: "lfpb115.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "Canada",
			Code: "CA",
			Cities: []VpnCity{
				{
					Name: "Canada",
					Code: "CA",
					Servers: []VpnServerNode{
						{Hostname: "ca.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "Japan",
			Code: "JP",
			Cities: []VpnCity{
				{
					Name: "Tokyo",
					Code: "RJTF",
					Servers: []VpnServerNode{
						{Hostname: "rjtf770.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "Singapore",
			Code: "SG",
			Cities: []VpnCity{
				{
					Name: "Singapore",
					Code: "WSAT",
					Servers: []VpnServerNode{
						{Hostname: "wsat188.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "Australia",
			Code: "AU",
			Cities: []VpnCity{
				{
					Name: "Sydney",
					Code: "YSBK",
					Servers: []VpnServerNode{
						{Hostname: "ysbk106.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "Sweden",
			Code: "SE",
			Cities: []VpnCity{
				{
					Name: "Stockholm",
					Code: "ESSB",
					Servers: []VpnServerNode{
						{Hostname: "essb127.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "Finland",
			Code: "FI",
			Cities: []VpnCity{
				{
					Name: "Helsinki",
					Code: "HEL",
					Servers: []VpnServerNode{
						{Hostname: "hel141.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "Spain",
			Code: "ES",
			Cities: []VpnCity{
				{
					Name: "Madrid",
					Code: "LETO",
					Servers: []VpnServerNode{
						{Hostname: "leto235.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "Italy",
			Code: "IT",
			Cities: []VpnCity{
				{
					Name: "Milan",
					Code: "MXP",
					Servers: []VpnServerNode{
						{Hostname: "mxp69.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "Switzerland",
			Code: "CH",
			Cities: []VpnCity{
				{
					Name: "Zurich",
					Code: "ZRH",
					Servers: []VpnServerNode{
						{Hostname: "zrh.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
		{
			Name: "Brazil",
			Code: "BR",
			Cities: []VpnCity{
				{
					Name: "Sao Paulo",
					Code: "GRU",
					Servers: []VpnServerNode{
						{Hostname: "gru.m1.fastly-masque.net", Port: 2499},
					},
				},
			},
		},
	}
}
