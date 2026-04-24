package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/auth"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/models"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/wgmgr"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/wgmgrclient"
)

type peersStore interface {
	Upsert(ctx context.Context, userID string, location string, wgPublicKey string, status string) (models.Peer, error)
}

type wgmgrProvisioner interface {
	CreatePeer(ctx context.Context, req wgmgrclient.CreatePeerRequest) (wgmgrclient.CreatePeerResponse, error)
}

type guestDevicesStore interface {
	ResolveUserID(ctx context.Context, deviceID string) (string, error)
}

type WireGuardHandler struct {
	peers    peersStore
	provSvc  wgmgrProvisioner
	guestMap guestDevicesStore
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

func NewWireGuardHandler(peers peersStore, provSvc wgmgrProvisioner, guestMap ...guestDevicesStore) *WireGuardHandler {
	var guests guestDevicesStore
	if len(guestMap) > 0 {
		guests = guestMap[0]
	}
	return &WireGuardHandler{peers: peers, provSvc: provSvc, guestMap: guests}
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

	req, ok := decodeWireGuardConfigRequest(w, r)
	if !ok {
		return
	}

	h.createConfigForUser(w, r, userID, req.Location)
}

type guestWireguardConfigRequest struct {
	DeviceID string `json:"deviceId"`
	Location string `json:"location"`
}

func (h *WireGuardHandler) CreateGuestConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.guestMap == nil {
		http.Error(w, "guest vpn is not configured", http.StatusNotImplemented)
		return
	}

	var req guestWireguardConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" {
		http.Error(w, "deviceId is required", http.StatusBadRequest)
		return
	}
	req.Location = strings.TrimSpace(req.Location)

	userID, err := h.guestMap.ResolveUserID(r.Context(), req.DeviceID)
	if err != nil {
		http.Error(w, "failed to resolve guest device", http.StatusInternalServerError)
		return
	}

	h.createConfigForUser(w, r, userID, req.Location)
}

func (h *WireGuardHandler) createConfigForUser(w http.ResponseWriter, r *http.Request, userID string, location string) {
	if location == "" {
		location = wgmgr.LocationFinland
	}
	profile, ok := wgmgr.ProfileForLocation(location)
	if !ok {
		http.Error(w, "unsupported location", http.StatusBadRequest)
		return
	}
	location = profile.Name

	mgmtResp, err := h.provSvc.CreatePeer(r.Context(), wgmgrclient.CreatePeerRequest{
		UserID:   userID,
		Location: location,
	})
	if err != nil {
		http.Error(w, "failed to provision peer", http.StatusBadGateway)
		return
	}

	if _, err := h.peers.Upsert(r.Context(), userID, location, mgmtResp.PublicKey, "active"); err != nil {
		http.Error(w, "failed to persist peer", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, wireguardConfigResponse{
		Location:  location,
		PeerID:    mgmtResp.PeerID,
		Address:   mgmtResp.Address,
		PublicKey: mgmtResp.PublicKey,
		Config:    mgmtResp.Config,
	})
}

func decodeWireGuardConfigRequest(w http.ResponseWriter, r *http.Request) (wireguardConfigRequest, bool) {
	var req wireguardConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return wireguardConfigRequest{}, false
	}
	req.Location = strings.TrimSpace(req.Location)
	return req, true
}
