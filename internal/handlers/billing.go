package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/auth"
	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/repositories"
)

type billingStore interface {
	ActivateProFromReceipt(
		ctx context.Context,
		userID string,
		productID string,
		platform string,
		purchaseToken string,
	) error
}

type BillingHandler struct {
	billing billingStore
}

func NewBillingHandler(billing billingStore) *BillingHandler {
	return &BillingHandler{billing: billing}
}

type verifyPurchaseRequest struct {
	ProductID     string `json:"productId"`
	PurchaseToken string `json:"purchaseToken"`
	Platform      string `json:"platform"`
}

func (h *BillingHandler) VerifyPurchase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req verifyPurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	req.ProductID = strings.TrimSpace(req.ProductID)
	req.PurchaseToken = strings.TrimSpace(req.PurchaseToken)
	req.Platform = strings.TrimSpace(strings.ToLower(req.Platform))
	if req.ProductID == "" || req.PurchaseToken == "" || req.Platform == "" {
		http.Error(w, "productId, purchaseToken, and platform are required", http.StatusBadRequest)
		return
	}
	if !isSupportedProductID(req.ProductID) {
		http.Error(w, "unsupported productId", http.StatusBadRequest)
		return
	}
	if req.Platform != "android" && req.Platform != "ios" {
		http.Error(w, "unsupported platform", http.StatusBadRequest)
		return
	}

	if err := h.billing.ActivateProFromReceipt(
		r.Context(),
		userID,
		req.ProductID,
		req.Platform,
		req.PurchaseToken,
	); err != nil {
		if errors.Is(err, repositories.ErrConflict) {
			http.Error(w, "purchase token is already linked to another user", http.StatusConflict)
			return
		}
		http.Error(w, "failed to verify purchase", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

func isSupportedProductID(productID string) bool {
	switch productID {
	case "mira_vpn_pro_monthly", "mira_vpn_pro_annual":
		return true
	default:
		return false
	}
}
