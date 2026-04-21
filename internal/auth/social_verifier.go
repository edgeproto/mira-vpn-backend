package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	ProviderGoogle = "google"
	ProviderApple  = "apple"
)

type SocialClaims struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
}

type SocialVerifier interface {
	VerifyGoogleIDToken(ctx context.Context, idToken string) (SocialClaims, error)
	VerifyAppleIDToken(ctx context.Context, idToken string) (SocialClaims, error)
}

type OIDCVerifier struct {
	httpClient      *http.Client
	googleAudiences map[string]struct{}
	appleAudiences  map[string]struct{}
}

func NewOIDCVerifier(googleAudiences []string, appleAudiences []string, timeout time.Duration) *OIDCVerifier {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &OIDCVerifier{
		httpClient:      &http.Client{Timeout: timeout},
		googleAudiences: toSet(googleAudiences),
		appleAudiences:  toSet(appleAudiences),
	}
}

func (v *OIDCVerifier) VerifyGoogleIDToken(ctx context.Context, idToken string) (SocialClaims, error) {
	if strings.TrimSpace(idToken) == "" {
		return SocialClaims{}, errors.New("missing google id token")
	}

	u := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return SocialClaims{}, err
	}
	res, err := v.httpClient.Do(req)
	if err != nil {
		return SocialClaims{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return SocialClaims{}, errors.New("invalid google token")
	}

	var payload struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified string `json:"email_verified"`
		Aud           string `json:"aud"`
		Iss           string `json:"iss"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return SocialClaims{}, err
	}
	if payload.Sub == "" {
		return SocialClaims{}, errors.New("google token missing subject")
	}
	if payload.Iss != "accounts.google.com" && payload.Iss != "https://accounts.google.com" {
		return SocialClaims{}, errors.New("invalid google issuer")
	}
	if len(v.googleAudiences) > 0 {
		if _, ok := v.googleAudiences[payload.Aud]; !ok {
			return SocialClaims{}, errors.New("invalid google audience")
		}
	}

	return SocialClaims{
		Provider:      ProviderGoogle,
		Subject:       payload.Sub,
		Email:         strings.ToLower(strings.TrimSpace(payload.Email)),
		EmailVerified: payload.EmailVerified == "true",
	}, nil
}

func (v *OIDCVerifier) VerifyAppleIDToken(ctx context.Context, idToken string) (SocialClaims, error) {
	if strings.TrimSpace(idToken) == "" {
		return SocialClaims{}, errors.New("missing apple id token")
	}

	token, err := jwt.Parse(idToken, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, errors.New("unexpected apple signing algorithm")
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("apple token missing kid")
		}
		return v.fetchApplePublicKey(ctx, kid)
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	if err != nil {
		return SocialClaims{}, err
	}
	if !token.Valid {
		return SocialClaims{}, errors.New("invalid apple token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return SocialClaims{}, errors.New("invalid apple claims")
	}
	iss, _ := claims["iss"].(string)
	if iss != "https://appleid.apple.com" {
		return SocialClaims{}, errors.New("invalid apple issuer")
	}

	aud, _ := claims["aud"].(string)
	if len(v.appleAudiences) > 0 {
		if _, ok := v.appleAudiences[aud]; !ok {
			return SocialClaims{}, errors.New("invalid apple audience")
		}
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return SocialClaims{}, errors.New("apple token missing subject")
	}
	email, _ := claims["email"].(string)
	emailVerified := false
	if raw, ok := claims["email_verified"].(string); ok {
		emailVerified = raw == "true"
	}
	if raw, ok := claims["email_verified"].(bool); ok {
		emailVerified = raw
	}

	return SocialClaims{
		Provider:      ProviderApple,
		Subject:       sub,
		Email:         strings.ToLower(strings.TrimSpace(email)),
		EmailVerified: emailVerified,
	}, nil
}

func (v *OIDCVerifier) fetchApplePublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://appleid.apple.com/auth/keys", nil)
	if err != nil {
		return nil, err
	}
	res, err := v.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch apple public keys")
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(res.Body).Decode(&jwks); err != nil {
		return nil, err
	}

	for _, key := range jwks.Keys {
		if key.Kid != kid {
			continue
		}
		if key.Kty != "RSA" {
			return nil, fmt.Errorf("unsupported apple key type %q", key.Kty)
		}
		return jwkToRSAPublicKey(key.N, key.E)
	}
	return nil, errors.New("matching apple key not found")
}

func jwkToRSAPublicKey(nBase64URL string, eBase64URL string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nBase64URL)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eBase64URL)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, errors.New("invalid rsa exponent")
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

func toSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result[trimmed] = struct{}{}
	}
	return result
}
