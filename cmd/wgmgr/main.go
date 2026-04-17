package main

import (
	"log"
	"net/http"
	"time"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/wgmgr"
)

func main() {
	cfg := wgmgr.LoadConfigFromEnv()

	if cfg.Mode != "mock" {
		log.Fatalf("unsupported WGMGR_MODE %q (only \"mock\" is implemented)", cfg.Mode)
	}

	prov, err := wgmgr.NewMockProvisioner(
		cfg.MockOutputDir,
		cfg.MockEndpoint,
		cfg.MockServerPubKey,
		cfg.MockDNS,
		cfg.MockAllowedIPs,
	)
	if err != nil {
		log.Fatal(err)
	}

	h := wgmgr.NewHandler(prov)
	mux := http.NewServeMux()
	h.Register(mux)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("wgmgr (mock) listening on %s, output %s", srv.Addr, cfg.MockOutputDir)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
