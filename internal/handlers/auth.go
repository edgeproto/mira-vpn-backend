package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"github.com/lib/pq"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/auth"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/models"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/repositories"
)

type usersStore interface {
	Create(ctx context.Context, email string, passwordHash string, isPro bool) (models.User, error)
	GetByEmail(ctx context.Context, email string) (models.User, error)
	GetByID(ctx context.Context, id string) (models.User, error)
	GetByProviderSubject(ctx context.Context, provider string, subject string) (models.User, error)
	CreateIdentity(ctx context.Context, userID string, provider string, providerSubject string, email string) error
}

type AuthHandler struct {
	users    usersStore
	tokens   *auth.TokenManager
	verifier auth.SocialVerifier
}

func NewAuthHandler(users usersStore, tokens *auth.TokenManager, verifier auth.SocialVerifier) *AuthHandler {
	return &AuthHandler{
		users:    users,
		tokens:   tokens,
		verifier: verifier,
	}
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	IsPro    bool   `json:"isPro"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

type socialAuthRequest struct {
	IDToken string `json:"idToken"`
	IsPro   bool   `json:"isPro"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req, ok := decodeAuthRequest(w, r)
	if !ok {
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	user, err := h.users.Create(r.Context(), req.Email, passwordHash, req.IsPro)
	if err != nil {
		if isUniqueViolation(err) {
			http.Error(w, "email already exists", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	token, err := h.tokens.CreateToken(user.ID)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{Token: token, User: user})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req, ok := decodeAuthRequest(w, r)
	if !ok {
		return
	}

	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		http.Error(w, "failed to fetch user", http.StatusInternalServerError)
		return
	}

	if err := auth.VerifyPassword(req.Password, user.PasswordHash); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := h.tokens.CreateToken(user.ID)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Error(w, "failed to fetch user", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *AuthHandler) SocialGoogle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.verifier == nil {
		http.Error(w, "social auth is not configured", http.StatusNotImplemented)
		return
	}

	req, ok := decodeSocialAuthRequest(w, r)
	if !ok {
		return
	}
	claims, err := h.verifier.VerifyGoogleIDToken(r.Context(), req.IDToken)
	if err != nil {
		http.Error(w, "invalid google token", http.StatusUnauthorized)
		return
	}

	h.completeSocialAuth(w, r, claims, req.IsPro)
}

func (h *AuthHandler) SocialApple(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.verifier == nil {
		http.Error(w, "social auth is not configured", http.StatusNotImplemented)
		return
	}

	req, ok := decodeSocialAuthRequest(w, r)
	if !ok {
		return
	}
	claims, err := h.verifier.VerifyAppleIDToken(r.Context(), req.IDToken)
	if err != nil {
		http.Error(w, "invalid apple token", http.StatusUnauthorized)
		return
	}

	h.completeSocialAuth(w, r, claims, req.IsPro)
}

func (h *AuthHandler) completeSocialAuth(w http.ResponseWriter, r *http.Request, claims auth.SocialClaims, isPro bool) {
	user, err := h.users.GetByProviderSubject(r.Context(), claims.Provider, claims.Subject)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		http.Error(w, "failed to fetch user", http.StatusInternalServerError)
		return
	}

	if errors.Is(err, repositories.ErrNotFound) {
		if claims.Email == "" {
			http.Error(w, "provider did not return an email", http.StatusBadRequest)
			return
		}

		userByEmail, lookupErr := h.users.GetByEmail(r.Context(), claims.Email)
		switch {
		case lookupErr == nil:
			user = userByEmail
		case errors.Is(lookupErr, repositories.ErrNotFound):
			user, lookupErr = h.users.Create(r.Context(), claims.Email, "", isPro)
			if lookupErr != nil {
				if isUniqueViolation(lookupErr) {
					http.Error(w, "email already exists", http.StatusConflict)
					return
				}
				http.Error(w, "failed to create user", http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, "failed to fetch user", http.StatusInternalServerError)
			return
		}

		if err := h.users.CreateIdentity(
			r.Context(),
			user.ID,
			claims.Provider,
			claims.Subject,
			claims.Email,
		); err != nil {
			http.Error(w, "failed to link social identity", http.StatusInternalServerError)
			return
		}
	}

	token, err := h.tokens.CreateToken(user.ID)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
}

func decodeAuthRequest(w http.ResponseWriter, r *http.Request) (authRequest, bool) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return authRequest{}, false
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		http.Error(w, "invalid email", http.StatusBadRequest)
		return authRequest{}, false
	}
	if len(req.Password) < 8 {
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return authRequest{}, false
	}

	return req, true
}

func decodeSocialAuthRequest(w http.ResponseWriter, r *http.Request) (socialAuthRequest, bool) {
	var req socialAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return socialAuthRequest{}, false
	}
	req.IDToken = strings.TrimSpace(req.IDToken)
	if req.IDToken == "" {
		http.Error(w, "idToken is required", http.StatusBadRequest)
		return socialAuthRequest{}, false
	}
	return req, true
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func isUniqueViolation(err error) bool {
	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
