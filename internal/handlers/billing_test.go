package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/auth"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/repositories"
)

type memoryBillingStore struct {
	seen map[string]string
}

func newMemoryBillingStore() *memoryBillingStore {
	return &memoryBillingStore{seen: map[string]string{}}
}

func (m *memoryBillingStore) ActivateProFromReceipt(
	_ context.Context,
	userID string,
	_ string,
	_ string,
	purchaseToken string,
) error {
	if existing, ok := m.seen[purchaseToken]; ok && existing != userID {
		return repositories.ErrConflict
	}
	m.seen[purchaseToken] = userID
	return nil
}

func TestBillingVerifyPurchase(t *testing.T) {
	tokens, err := auth.NewTokenManager("test-secret", "test-issuer", 5*time.Minute)
	if err != nil {
		t.Fatalf("token manager init failed: %v", err)
	}
	userToken, err := tokens.CreateToken("user-1")
	if err != nil {
		t.Fatalf("create token failed: %v", err)
	}

	store := newMemoryBillingStore()
	handler := NewBillingHandler(store)
	mux := http.NewServeMux()
	mux.Handle("/billing/verify", auth.Middleware(tokens)(http.HandlerFunc(handler.VerifyPurchase)))

	body := map[string]string{
		"productId":     "mira_vpn_pro_monthly",
		"purchaseToken": "purchase-abc",
		"platform":      "android",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/billing/verify", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+userToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestBillingVerifyPurchaseRejectsUnknownProduct(t *testing.T) {
	tokens, err := auth.NewTokenManager("test-secret", "test-issuer", 5*time.Minute)
	if err != nil {
		t.Fatalf("token manager init failed: %v", err)
	}
	userToken, err := tokens.CreateToken("user-1")
	if err != nil {
		t.Fatalf("create token failed: %v", err)
	}

	handler := NewBillingHandler(newMemoryBillingStore())
	mux := http.NewServeMux()
	mux.Handle("/billing/verify", auth.Middleware(tokens)(http.HandlerFunc(handler.VerifyPurchase)))

	body := map[string]string{
		"productId":     "unknown",
		"purchaseToken": "purchase-abc",
		"platform":      "android",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/billing/verify", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+userToken)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rr.Code, rr.Body.String())
	}
}
