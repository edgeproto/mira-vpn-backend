package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/auth"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/models"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/repositories"
)

type memoryUsersStore struct {
	byID    map[string]models.User
	byEmail map[string]models.User
	byIdent map[string]string
	nextID  int
}

func newMemoryUsersStore() *memoryUsersStore {
	return &memoryUsersStore{
		byID:    map[string]models.User{},
		byEmail: map[string]models.User{},
		byIdent: map[string]string{},
		nextID:  1,
	}
}

func (m *memoryUsersStore) Create(_ context.Context, email string, passwordHash string, isPro bool) (models.User, error) {
	if _, exists := m.byEmail[email]; exists {
		return models.User{}, errors.New("duplicate email")
	}

	id := "user-" + strconvItoa(m.nextID)
	m.nextID++
	user := models.User{
		ID:           id,
		Email:        email,
		PasswordHash: passwordHash,
		IsPro:        isPro,
		CreatedAt:    time.Now(),
	}
	m.byID[id] = user
	m.byEmail[email] = user
	return user, nil
}

func (m *memoryUsersStore) GetByEmail(_ context.Context, email string) (models.User, error) {
	user, ok := m.byEmail[email]
	if !ok {
		return models.User{}, repositories.ErrNotFound
	}
	return user, nil
}

func (m *memoryUsersStore) GetByID(_ context.Context, id string) (models.User, error) {
	user, ok := m.byID[id]
	if !ok {
		return models.User{}, repositories.ErrNotFound
	}
	return user, nil
}

func (m *memoryUsersStore) GetByProviderSubject(_ context.Context, provider string, subject string) (models.User, error) {
	userID, ok := m.byIdent[provider+":"+subject]
	if !ok {
		return models.User{}, repositories.ErrNotFound
	}
	user, ok := m.byID[userID]
	if !ok {
		return models.User{}, repositories.ErrNotFound
	}
	return user, nil
}

func (m *memoryUsersStore) CreateIdentity(
	_ context.Context,
	userID string,
	provider string,
	providerSubject string,
	_ string,
) error {
	m.byIdent[provider+":"+providerSubject] = userID
	return nil
}

func TestAuthRegisterLoginAndProtectedFlow(t *testing.T) {
	users := newMemoryUsersStore()
	tokens, err := auth.NewTokenManager("test-secret", "test-issuer", 5*time.Minute)
	if err != nil {
		t.Fatalf("expected token manager creation to succeed: %v", err)
	}
	authHandler := NewAuthHandler(users, tokens, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/register", authHandler.Register)
	mux.HandleFunc("/auth/login", authHandler.Login)
	mux.Handle("/auth/me", auth.Middleware(tokens)(http.HandlerFunc(authHandler.Me)))
	server := httptest.NewServer(mux)
	defer server.Close()

	registerBody := strings.NewReader(`{"email":"alice@example.com","password":"password123"}`)
	registerRes, err := http.Post(server.URL+"/auth/register", "application/json", registerBody)
	if err != nil {
		t.Fatalf("expected register request to succeed: %v", err)
	}
	defer registerRes.Body.Close()

	if registerRes.StatusCode != http.StatusCreated {
		t.Fatalf("expected register status %d, got %d", http.StatusCreated, registerRes.StatusCode)
	}

	loginBody := strings.NewReader(`{"email":"alice@example.com","password":"password123"}`)
	loginRes, err := http.Post(server.URL+"/auth/login", "application/json", loginBody)
	if err != nil {
		t.Fatalf("expected login request to succeed: %v", err)
	}
	defer loginRes.Body.Close()

	if loginRes.StatusCode != http.StatusOK {
		t.Fatalf("expected login status %d, got %d", http.StatusOK, loginRes.StatusCode)
	}

	var authPayload struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginRes.Body).Decode(&authPayload); err != nil {
		t.Fatalf("expected login response to decode: %v", err)
	}
	if authPayload.Token == "" {
		t.Fatalf("expected non-empty token")
	}

	meReq, _ := http.NewRequest(http.MethodGet, server.URL+"/auth/me", bytes.NewReader(nil))
	meReq.Header.Set("Authorization", "Bearer "+authPayload.Token)
	meRes, err := http.DefaultClient.Do(meReq)
	if err != nil {
		t.Fatalf("expected protected request to succeed: %v", err)
	}
	defer meRes.Body.Close()

	if meRes.StatusCode != http.StatusOK {
		t.Fatalf("expected /auth/me status %d, got %d", http.StatusOK, meRes.StatusCode)
	}

	var me models.User
	if err := json.NewDecoder(meRes.Body).Decode(&me); err != nil {
		t.Fatalf("expected /auth/me response to decode: %v", err)
	}
	if me.Email != "alice@example.com" {
		t.Fatalf("expected /auth/me email alice@example.com, got %q", me.Email)
	}
}

func strconvItoa(v int) string {
	return fmt.Sprintf("%d", v)
}
