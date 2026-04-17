package wgmgr

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

// Handler serves the WireGuard management HTTP API (mock mode).
type Handler struct {
	provision *MockProvisioner
}

// NewHandler returns an HTTP handler for mock provisioning.
func NewHandler(p *MockProvisioner) *Handler {
	return &Handler{provision: p}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /v1/peers", h.createPeer)
	mux.HandleFunc("DELETE /v1/peers/{peerID}", h.deletePeer)
}

func health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

type createPeerRequest struct {
	UserID   string `json:"userId"`
	Location string `json:"location"`
}

type createPeerResponse struct {
	PeerID     string `json:"peerId"`
	UserID     string `json:"userId"`
	Location   string `json:"location"`
	PublicKey  string `json:"publicKey"`
	Address    string `json:"address"`
	ConfigPath string `json:"configPath"`
}

func (h *Handler) createPeer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req createPeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Location == "" {
		req.Location = "USA"
	}

	meta, err := h.provision.CreatePeer(req.UserID, req.Location)
	if err != nil {
		log.Printf("create peer: %v", err)
		http.Error(w, `{"error":"provision failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createPeerResponse{
		PeerID:     meta.PeerID,
		UserID:     meta.UserID,
		Location:   meta.Location,
		PublicKey:  meta.PublicKey,
		Address:    meta.Address,
		ConfigPath: meta.ConfigPath,
	})
}

func (h *Handler) deletePeer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	raw := r.PathValue("peerID")
	id, err := ParsePeerID(raw)
	if err != nil {
		http.Error(w, `{"error":"invalid peer id"}`, http.StatusBadRequest)
		return
	}

	err = h.provision.DeletePeer(id)
	if errors.Is(err, ErrNotFound) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("delete peer: %v", err)
		http.Error(w, `{"error":"delete failed"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
