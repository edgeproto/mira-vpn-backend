package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/auth"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/models"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/repositories"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/wgmgrclient"
)

type memoryPeersStore struct {
	byKey map[string]models.Peer
}

func newMemoryPeersStore() *memoryPeersStore {
	return &memoryPeersStore{byKey: map[string]models.Peer{}}
}

func (m *memoryPeersStore) Create(_ context.Context, userID string, location string, wgPublicKey string, status string) (models.Peer, error) {
	key := userID + "|" + location
	if _, ok := m.byKey[key]; ok {
		return models.Peer{}, errors.New("duplicate")
	}
	p := models.Peer{
		ID:          "peer-db-1",
		UserID:      userID,
		Location:    location,
		WgPublicKey: wgPublicKey,
		Status:      status,
		CreatedAt:   time.Now(),
	}
	m.byKey[key] = p
	return p, nil
}

func (m *memoryPeersStore) GetByUserAndLocation(_ context.Context, userID string, location string) (models.Peer, error) {
	key := userID + "|" + location
	p, ok := m.byKey[key]
	if !ok {
		return models.Peer{}, repositories.ErrNotFound
	}
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
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, resp.StatusCode)
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
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, resp.StatusCode)
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
