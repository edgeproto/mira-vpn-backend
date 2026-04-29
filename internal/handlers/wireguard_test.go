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
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/models"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/wgmgr"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/wgmgrclient"
)

type memoryPeersStore struct {
	byKey map[string]models.Peer
}

func newMemoryPeersStore() *memoryPeersStore {
	return &memoryPeersStore{byKey: map[string]models.Peer{}}
}

func (m *memoryPeersStore) Upsert(_ context.Context, userID string, location string, wgPublicKey string, status string) (models.Peer, error) {
	key := userID + "|" + location
	existing, ok := m.byKey[key]
	id := "peer-db-1"
	if ok && existing.ID != "" {
		id = existing.ID
	}
	p := models.Peer{
		ID:          id,
		UserID:      userID,
		Location:    location,
		WgPublicKey: wgPublicKey,
		Status:      status,
		CreatedAt:   time.Now(),
	}
	if ok {
		p.CreatedAt = existing.CreatedAt
	}
	m.byKey[key] = p
	return p, nil
}

type stubProvisioner struct {
	resp wgmgrclient.CreatePeerResponse
	err  error
}

type memoryGuestStore struct {
	byDevice map[string]string
	nextID   int
}

func newMemoryGuestStore() *memoryGuestStore {
	return &memoryGuestStore{
		byDevice: map[string]string{},
		nextID:   100,
	}
}

func (m *memoryGuestStore) ResolveUserID(_ context.Context, deviceID string) (string, error) {
	if id, ok := m.byDevice[deviceID]; ok {
		return id, nil
	}
	id := "guest-user-" + strconvItoa(m.nextID)
	m.nextID++
	m.byDevice[deviceID] = id
	return id, nil
}

func (s stubProvisioner) CreatePeer(_ context.Context, _ wgmgrclient.CreatePeerRequest) (wgmgrclient.CreatePeerResponse, error) {
	if s.err != nil {
		return wgmgrclient.CreatePeerResponse{}, s.err
	}
	return s.resp, nil
}

func TestWireGuardListLocations(t *testing.T) {
	t.Setenv("WGMGR_LOCATION_PROFILES_JSON", `[
		{"name":"Germany","endpoint":"de.example.com:443","serverPublicKey":"de-pub"},
		{"name":"Finland","endpoint":"fi.example.com:443","serverPublicKey":"fi-pub"}
	]`)
	if err := wgmgr.LoadLocationProfilesFromEnv(); err != nil {
		t.Fatalf("load location profiles: %v", err)
	}
	t.Cleanup(func() {
		t.Setenv("WGMGR_LOCATION_PROFILES_JSON", "")
		_ = wgmgr.LoadLocationProfilesFromEnv()
	})

	h := NewWireGuardHandler(newMemoryPeersStore(), stubProvisioner{})
	req := httptest.NewRequest(http.MethodGet, "/wireguard/locations", nil)
	rec := httptest.NewRecorder()

	h.ListLocations(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var out struct {
		Locations []struct {
			Name string `json:"name"`
		} `json:"locations"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Locations) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(out.Locations))
	}
	if out.Locations[0].Name != "Finland" || out.Locations[1].Name != "Germany" {
		t.Fatalf("unexpected order/content: %+v", out.Locations)
	}
}

func TestWireGuardCreateConfig_ProtectedFlow(t *testing.T) {
	t.Parallel()

	tokens, err := auth.NewTokenManager("test-secret", "test-issuer", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	token, err := tokens.CreateToken("user-1")
	if err != nil {
		t.Fatal(err)
	}

	peers := newMemoryPeersStore()
	h := NewWireGuardHandler(peers, stubProvisioner{
		resp: wgmgrclient.CreatePeerResponse{
			PeerID:    "abcd1234",
			PublicKey: "wg-public-key",
			Address:   "10.200.0.2/32",
			Config:    "[Interface]\nPrivateKey = test\n",
		},
	})

	mux := http.NewServeMux()
	mux.Handle("/wireguard/config", auth.Middleware(tokens)(http.HandlerFunc(h.CreateConfig)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	reqBody := bytes.NewBufferString(`{"location":"Finland"}`)
	req, err := http.NewRequest(http.MethodPost, server.URL+"/wireguard/config", reqBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var out struct {
		Location string `json:"location"`
		PeerID   string `json:"peerId"`
		Config   string `json:"config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Location != "Finland" || out.PeerID == "" || out.Config == "" {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestWireGuardCreateConfig_IdempotentSecondRequest(t *testing.T) {
	t.Parallel()

	tokens, err := auth.NewTokenManager("test-secret", "test-issuer", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	token, err := tokens.CreateToken("user-1")
	if err != nil {
		t.Fatal(err)
	}

	peers := newMemoryPeersStore()
	h := NewWireGuardHandler(peers, stubProvisioner{
		resp: wgmgrclient.CreatePeerResponse{
			PeerID:    "abcd1234",
			PublicKey: "wg-public-key",
			Address:   "10.200.0.2/32",
			Config:    "[Interface]\nPrivateKey = test\n",
		},
	})

	mux := http.NewServeMux()
	mux.Handle("/wireguard/config", auth.Middleware(tokens)(http.HandlerFunc(h.CreateConfig)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	do := func() *http.Response {
		t.Helper()
		reqBody := bytes.NewBufferString(`{"location":"Finland"}`)
		req, err := http.NewRequest(http.MethodPost, server.URL+"/wireguard/config", reqBody)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	r1 := do()
	defer r1.Body.Close()
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("first: expected %d, got %d", http.StatusOK, r1.StatusCode)
	}

	r2 := do()
	defer r2.Body.Close()
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("second: expected %d, got %d", http.StatusOK, r2.StatusCode)
	}
}

func TestWireGuardCreateGuestConfig(t *testing.T) {
	t.Parallel()

	peers := newMemoryPeersStore()
	guests := newMemoryGuestStore()
	h := NewWireGuardHandler(peers, stubProvisioner{
		resp: wgmgrclient.CreatePeerResponse{
			PeerID:    "guest-peer-1",
			PublicKey: "wg-public-key",
			Address:   "10.200.0.3/32",
			Config:    "[Interface]\nPrivateKey = test\n",
		},
	}, guests)

	mux := http.NewServeMux()
	mux.HandleFunc("/wireguard/config/guest", h.CreateGuestConfig)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	reqBody := bytes.NewBufferString(`{"deviceId":"device-abc","location":"Finland"}`)
	resp, err := http.Post(server.URL+"/wireguard/config/guest", "application/json", reqBody)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var out struct {
		PeerID string `json:"peerId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.PeerID == "" {
		t.Fatalf("expected peerId in response")
	}
}
