package wgmgr_test

import (
	"strings"
	"testing"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/wgmgr"
)

func TestBuildClientConfig_GenericTemplateFields(t *testing.T) {
	t.Parallel()

	cfg := wgmgr.BuildClientConfig(wgmgr.ClientConfigInput{
		ClientPrivateKey: "private-key",
		ClientAddress:    "10.200.0.2/32",
		DNS:              "1.1.1.1",
		ServerPublicKey:  "server-public-key",
		Endpoint:         "fi.mira-vpn.example:51820",
		AllowedIPs:       "0.0.0.0/0, ::/0",
		Keepalive:        25,
	})

	required := []string{
		"[Interface]",
		"PrivateKey = private-key",
		"Address = 10.200.0.2/32",
		"DNS = 1.1.1.1",
		"[Peer]",
		"PublicKey = server-public-key",
		"Endpoint = fi.mira-vpn.example:51820",
		"AllowedIPs = 0.0.0.0/0, ::/0",
		"PersistentKeepalive = 25",
	}
	for _, r := range required {
		if !strings.Contains(cfg, r) {
			t.Fatalf("expected config to contain %q, got:\n%s", r, cfg)
		}
	}
}

func TestBuildClientConfig_DefaultAllowedIPs(t *testing.T) {
	t.Parallel()

	cfg := wgmgr.BuildClientConfig(wgmgr.ClientConfigInput{
		ClientPrivateKey: "private-key",
		ClientAddress:    "10.200.0.2/32",
		ServerPublicKey:  "server-public-key",
		Endpoint:         "fi.mira-vpn.example:51820",
	})

	if !strings.Contains(cfg, "AllowedIPs = 0.0.0.0/0, ::/0") {
		t.Fatalf("expected default allowed IPs, got:\n%s", cfg)
	}
}

func TestProfileForLocation_DefaultFinlandProfile(t *testing.T) {
	t.Parallel()

	profile, ok := wgmgr.ProfileForLocation("finland")
	if !ok {
		t.Fatalf("expected finland profile")
	}
	if profile.Name != wgmgr.LocationFinland {
		t.Fatalf("expected name %q, got %q", wgmgr.LocationFinland, profile.Name)
	}
	if profile.AllowedIPs != wgmgr.DefaultAllowedIPs {
		t.Fatalf("expected default allowed IPs, got %q", profile.AllowedIPs)
	}
	if profile.Keepalive != 25 {
		t.Fatalf("expected keepalive 25, got %d", profile.Keepalive)
	}
}
