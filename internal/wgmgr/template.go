package wgmgr

import (
	"strconv"
	"strings"
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

// Profiles stores known location profiles and can be extended over time.
var Profiles = map[string]LocationProfile{
	LocationFinland: {
		Name:      LocationFinland,
		Keepalive: 25,
	},
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
	name := strings.TrimSpace(location)
	for key, profile := range Profiles {
		if strings.EqualFold(name, key) {
			profile.Name = key
			if profile.AllowedIPs == "" {
				profile.AllowedIPs = DefaultAllowedIPs
			}
			if profile.Keepalive <= 0 {
				profile.Keepalive = 25
			}
			return profile, true
		}
	}
	return LocationProfile{}, false
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
