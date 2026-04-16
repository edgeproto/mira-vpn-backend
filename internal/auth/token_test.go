package auth

import (
	"testing"
	"time"
)

func TestTokenManagerCreateAndParse(t *testing.T) {
	tokens, err := NewTokenManager("test-secret", "test-issuer", 2*time.Minute)
	if err != nil {
		t.Fatalf("expected manager creation to succeed: %v", err)
	}

	raw, err := tokens.CreateToken("user-123")
	if err != nil {
		t.Fatalf("expected token creation to succeed: %v", err)
	}

	claims, err := tokens.ParseToken(raw)
	if err != nil {
		t.Fatalf("expected token parse to succeed: %v", err)
	}

	if claims.Subject != "user-123" {
		t.Fatalf("expected subject user-123, got %q", claims.Subject)
	}
	if claims.Issuer != "test-issuer" {
		t.Fatalf("expected issuer test-issuer, got %q", claims.Issuer)
	}
	if claims.ExpiresAt == nil {
		t.Fatalf("expected token expiry to be set")
	}
}
