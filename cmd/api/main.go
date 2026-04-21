package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/auth"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/db"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/handlers"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/repositories"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/wgmgrclient"
)

func main() {
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}

	dbConn, err := db.Open(context.Background(), db.DefaultConfig())
	if err != nil {
		log.Fatal(err)
	}
	defer dbConn.Close()

	tokenTTLMinutes := getEnvInt("JWT_TTL_MINUTES", 15)
	tokenManager, err := auth.NewTokenManager(
		getEnv("JWT_SECRET", "dev-secret-change-me"),
		getEnv("JWT_ISSUER", "mira-vpn-api"),
		time.Duration(tokenTTLMinutes)*time.Minute,
	)
	if err != nil {
		log.Fatal(err)
	}
	verifier := auth.NewOIDCVerifier(
		splitEnvCSV("GOOGLE_CLIENT_IDS"),
		splitEnvCSV("APPLE_AUDIENCES"),
		time.Duration(getEnvInt("OIDC_TIMEOUT_SECONDS", 5))*time.Second,
	)

	usersRepo := repositories.NewUsersRepository(dbConn)
	peersRepo := repositories.NewPeersRepository(dbConn)
	authHandler := handlers.NewAuthHandler(usersRepo, tokenManager, verifier)
	wgmgrClient := wgmgrclient.New(
		getEnv("WGMGR_BASE_URL", "http://127.0.0.1:9090"),
		time.Duration(getEnvInt("WGMGR_TIMEOUT_SECONDS", 5))*time.Second,
	)
	wireGuardHandler := handlers.NewWireGuardHandler(peersRepo, wgmgrClient)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handlers.Health)
	mux.HandleFunc("/auth/register", authHandler.Register)
	mux.HandleFunc("/auth/login", authHandler.Login)
	mux.HandleFunc("/auth/social/google", authHandler.SocialGoogle)
	mux.HandleFunc("/auth/social/apple", authHandler.SocialApple)
	mux.Handle("/auth/me", auth.Middleware(tokenManager)(http.HandlerFunc(authHandler.Me)))
	mux.Handle("/wireguard/config", auth.Middleware(tokenManager)(http.HandlerFunc(wireGuardHandler.CreateConfig)))

	srv := &http.Server{
		Addr:              ":" + addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("api listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func getEnv(key string, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func splitEnvCSV(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return values
}
