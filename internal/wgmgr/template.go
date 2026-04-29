package wgmgr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// IPv4-only default: many VPN hosts only NAT IPv4; routing ::/0 without v6 NAT
// can stall clients on AAAA-heavy sites. Use WGMGR_*_ALLOWED_IPS when v6 is ready.
const DefaultAllowedIPs = "0.0.0.0/0"

const (
	LocationFinland = "Finland"
)

// LocationProfile contains location-specific WireGuard server settings.
type LocationProfile struct {
	Name            string
	Endpoint        string
	ServerPublicKey string
	DNS             string
	AllowedIPs      string
	Keepalive       int
}

var (
	profilesMu sync.RWMutex
	profiles   = defaultLocationProfiles()
)

type profileInput struct {
	Name            string `json:"name"`
	Endpoint        string `json:"endpoint"`
	ServerPublicKey string `json:"serverPublicKey"`
	DNS             string `json:"dns"`
	AllowedIPs      string `json:"allowedIPs"`
	Keepalive       int    `json:"keepalive"`
}

// ClientConfigInput contains values required to render a client config.
type ClientConfigInput struct {
	ClientPrivateKey string
	ClientAddress    string
	DNS              string
	// MTU is written under [Interface] when > 0 (helps path-MTU / TLS stalls on some networks).
	MTU             int
	ServerPublicKey string
	Endpoint        string
	AllowedIPs      string
	Keepalive       int
}

// ProfileForLocation returns a normalized profile for a location.
func ProfileForLocation(location string) (LocationProfile, bool) {
	profilesMu.RLock()
	defer profilesMu.RUnlock()

	key := normalizeKey(location)
	profile, ok := profiles[key]
	if !ok {
		return LocationProfile{}, false
	}
	normalized, err := normalizeProfile(profile.Name, profile)
	if err != nil {
		return LocationProfile{}, false
	}
	return normalized, true
}

// LoadLocationProfilesFromEnv loads profiles from WGMGR_LOCATION_PROFILES_JSON.
// Empty/whitespace value resets to the built-in default (Finland).
func LoadLocationProfilesFromEnv() error {
	raw := strings.TrimSpace(os.Getenv("WGMGR_LOCATION_PROFILES_JSON"))
	if raw == "" {
		profilesMu.Lock()
		profiles = defaultLocationProfiles()
		profilesMu.Unlock()
		return nil
	}
	return LoadLocationProfilesJSON(raw)
}

// LoadLocationProfilesJSON replaces active location profiles from JSON.
// JSON shape: [{"name":"Finland","endpoint":"fi.example:443", ...}]
func LoadLocationProfilesJSON(raw string) error {
	next, err := ParseLocationProfilesJSON(raw)
	if err != nil {
		return err
	}
	profilesMu.Lock()
	profiles = next
	profilesMu.Unlock()
	return nil
}

// ParseLocationProfilesJSON parses and normalizes location profiles JSON without
// mutating active runtime profiles.
func ParseLocationProfilesJSON(raw string) (map[string]LocationProfile, error) {
	var decoded []profileInput
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, fmt.Errorf("wgmgr: parse WGMGR_LOCATION_PROFILES_JSON: %w", err)
	}
	if len(decoded) == 0 {
		return nil, errors.New("wgmgr: location profiles list is empty")
	}

	next := make(map[string]LocationProfile, len(decoded))
	for _, in := range decoded {
		p, err := normalizeProfile(in.Name, LocationProfile{
			Name:            in.Name,
			Endpoint:        in.Endpoint,
			ServerPublicKey: in.ServerPublicKey,
			DNS:             in.DNS,
			AllowedIPs:      in.AllowedIPs,
			Keepalive:       in.Keepalive,
		})
		if err != nil {
			return nil, err
		}
		next[normalizeKey(p.Name)] = p
	}
	return next, nil
}

func defaultLocationProfiles() map[string]LocationProfile {
	p, err := normalizeProfile(LocationFinland, LocationProfile{
		Name:      LocationFinland,
		Keepalive: 25,
	})
	if err != nil {
		// unreachable with hardcoded values
		panic(err)
	}
	return map[string]LocationProfile{
		normalizeKey(LocationFinland): p,
	}
}

func normalizeKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func normalizeProfile(name string, profile LocationProfile) (LocationProfile, error) {
	canonicalName := strings.TrimSpace(name)
	if canonicalName == "" {
		return LocationProfile{}, errors.New("wgmgr: location profile name is required")
	}
	profile.Name = canonicalName
	profile.Endpoint = strings.TrimSpace(profile.Endpoint)
	profile.ServerPublicKey = strings.TrimSpace(profile.ServerPublicKey)
	profile.DNS = strings.TrimSpace(profile.DNS)
	profile.AllowedIPs = strings.TrimSpace(profile.AllowedIPs)
	if profile.AllowedIPs == "" {
		profile.AllowedIPs = DefaultAllowedIPs
	}
	if profile.Keepalive <= 0 {
		profile.Keepalive = 25
	}
	return profile, nil
}

// BuildClientConfig renders a WireGuard client config from input fields.
func BuildClientConfig(in ClientConfigInput) string {
	allowedIPs := strings.TrimSpace(in.AllowedIPs)
	if allowedIPs == "" {
		allowedIPs = DefaultAllowedIPs
	}
	keepalive := in.Keepalive
	if keepalive <= 0 {
		keepalive = 25
	}

	var b strings.Builder
	b.WriteString("[Interface]\n")
	b.WriteString("PrivateKey = ")
	b.WriteString(in.ClientPrivateKey)
	b.WriteString("\n")
	b.WriteString("Address = ")
	b.WriteString(in.ClientAddress)
	b.WriteString("\n")
	if in.MTU > 0 {
		b.WriteString("MTU = ")
		b.WriteString(strconv.Itoa(in.MTU))
		b.WriteString("\n")
	}
	if in.DNS != "" {
		b.WriteString("DNS = ")
		b.WriteString(in.DNS)
		b.WriteString("\n")
	}
	b.WriteString("\n[Peer]\n")
	b.WriteString("PublicKey = ")
	b.WriteString(in.ServerPublicKey)
	b.WriteString("\n")
	b.WriteString("Endpoint = ")
	b.WriteString(in.Endpoint)
	b.WriteString("\n")
	b.WriteString("AllowedIPs = ")
	b.WriteString(allowedIPs)
	b.WriteString("\n")
	b.WriteString("PersistentKeepalive = ")
	b.WriteString(strconv.Itoa(keepalive))
	b.WriteString("\n")
	return b.String()
}
