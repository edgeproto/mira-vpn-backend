package wgmgr

import "os"

// Config holds WireGuard management service settings (mock mode).
//
// Env:
//   - PORT: listen port (default "9090")
//   - WGMGR_MODE: "mock" (default) — write configs only, no wg set
//   - WGMGR_MOCK_OUTPUT_DIR: directory for peer .conf and metadata (default ./var/wgmgr-mock)
//   - WGMGR_MOCK_ENDPOINT: WireGuard server endpoint host:port for client configs
//   - WGMGR_MOCK_SERVER_PUBLIC_KEY: server WireGuard public key (base64)
//   - WGMGR_MOCK_DNS: optional DNS line in [Interface] (e.g. 1.1.1.1)
type Config struct {
	Port             string
	Mode             string
	MockOutputDir    string
	MockEndpoint     string
	MockServerPubKey string
	MockDNS          string
}

func LoadConfigFromEnv() Config {
	return Config{
		Port:             getEnv("PORT", "9090"),
		Mode:             getEnv("WGMGR_MODE", "mock"),
		MockOutputDir:    getEnv("WGMGR_MOCK_OUTPUT_DIR", "var/wgmgr-mock"),
		MockEndpoint:     getEnv("WGMGR_MOCK_ENDPOINT", "127.0.0.1:51820"),
		MockServerPubKey: getEnv("WGMGR_MOCK_SERVER_PUBLIC_KEY", DefaultMockServerPublicKey),
		MockDNS:          os.Getenv("WGMGR_MOCK_DNS"),
	}
}

// DefaultMockServerPublicKey is a valid WireGuard public key used when
// WGMGR_MOCK_SERVER_PUBLIC_KEY is unset (local dev).
const DefaultMockServerPublicKey = "BB/7/1u13mBwC2kWQyEnKcuU1z9MChg3QiJjezAmujo="

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
