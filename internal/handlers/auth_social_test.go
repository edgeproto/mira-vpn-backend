package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/auth"
)

type fakeSocialVerifier struct {
	claims auth.SocialClaims
}

func (f fakeSocialVerifier) VerifyGoogleIDToken(_ context.Context, _ string) (auth.SocialClaims, error) {
	return f.claims, nil
}

func (f fakeSocialVerifier) VerifyAppleIDToken(_ context.Context, _ string) (auth.SocialClaims, error) {
	return f.claims, nil
}

func TestSocialGoogleCreatesAccount(t *testing.T) {
	users := newMemoryUsersStore()
	tokens, err := auth.NewTokenManager("test-secret", "test-issuer", 5*time.Minute)
	if err != nil {
		t.Fatalf("expected token manager creation to succeed: %v", err)
	}
	verifier := fakeSocialVerifier{
		claims: auth.SocialClaims{
			Provider:      auth.ProviderGoogle,
			Subject:       "google-sub-1",
			Email:         "social@example.com",
			EmailVerified: true,
		},
	}
	authHandler := NewAuthHandler(users, tokens, verifier)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/social/google", authHandler.SocialGoogle)
	server := httptest.NewServer(mux)
	defer server.Close()

	body := strings.NewReader(`{"idToken":"dummy"}`)
	res, err := http.Post(server.URL+"/auth/social/google", "application/json", body)
	if err != nil {
		t.Fatalf("expected social login request to succeed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected social login status %d, got %d", http.StatusOK, res.StatusCode)
	}

	var payload struct {
		Token string `json:"token"`
		User  struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("expected social login response to decode: %v", err)
	}
	if payload.Token == "" {
		t.Fatalf("expected social login token to be non-empty")
	}
	if payload.User.Email != "social@example.com" {
		t.Fatalf("expected social login email social@example.com, got %q", payload.User.Email)
	}
}
