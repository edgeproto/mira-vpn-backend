package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/auth"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/models"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/repositories"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/wgmgr"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/wgmgrclient"
)

type peersStore interface {
	Create(ctx context.Context, userID string, location string, wgPublicKey string, status string) (models.Peer, error)
	GetByUserAndLocation(ctx context.Context, userID string, location string) (models.Peer, error)
}

type wgmgrProvisioner interface {
	CreatePeer(ctx context.Context, req wgmgrclient.CreatePeerRequest) (wgmgrclient.CreatePeerResponse, error)
}

type WireGuardHandler struct {
	peers   peersStore
	provSvc wgmgrProvisioner
}

type wireguardConfigRequest struct {
	Location string `json:"location"`
}

type wireguardConfigResponse struct {
	Location  string `json:"location"`
	PeerID    string `json:"peerId"`
	Address   string `json:"address"`
	PublicKey string `json:"publicKey"`
	Config    string `json:"config"`
}

func NewWireGuardHandler(peers peersStore, provSvc wgmgrProvisioner) *WireGuardHandler {
	return &WireGuardHandler{peers: peers, provSvc: provSvc}
}

func (h *WireGuardHandler) CreateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req wireguardConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	location := strings.TrimSpace(req.Location)
	if location == "" {
		location = wgmgr.LocationFinland
	}
	profile, ok := wgmgr.ProfileForLocation(location)
	if !ok {
		http.Error(w, "unsupported location", http.StatusBadRequest)
		return
	}
	location = profile.Name

	_, err := h.peers.GetByUserAndLocation(r.Context(), userID, location)
	if err == nil {
		http.Error(w, "peer already exists for location", http.StatusConflict)
		return
	}
	if !errors.Is(err, repositories.ErrNotFound) {
		http.Error(w, "failed to check existing peer", http.StatusInternalServerError)
		return
	}

	mgmtResp, err := h.provSvc.CreatePeer(r.Context(), wgmgrclient.CreatePeerRequest{
		UserID:   userID,
		Location: location,
	})
	if err != nil {
		http.Error(w, "failed to provision peer", http.StatusBadGateway)
		return
	}

	_, err = h.peers.Create(r.Context(), userID, location, mgmtResp.PublicKey, "active")
	if err != nil {
		if isUniqueViolation(err) {
			http.Error(w, "peer already exists for location", http.StatusConflict)
			return
		}
		http.Error(w, "failed to persist peer", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, wireguardConfigResponse{
		Location:  location,
		PeerID:    mgmtResp.PeerID,
		Address:   mgmtResp.Address,
		PublicKey: mgmtResp.PublicKey,
		Config:    mgmtResp.Config,
	})
}
