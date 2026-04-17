package wgmgrclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type CreatePeerRequest struct {
	UserID   string `json:"userId"`
	Location string `json:"location"`
}

type CreatePeerResponse struct {
	PeerID    string `json:"peerId"`
	PublicKey string `json:"publicKey"`
	Address   string `json:"address"`
	Config    string `json:"config"`
}

func New(baseURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) CreatePeer(ctx context.Context, req CreatePeerRequest) (CreatePeerResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return CreatePeerResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/peers", bytes.NewReader(body))
	if err != nil {
		return CreatePeerResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return CreatePeerResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return CreatePeerResponse{}, fmt.Errorf("wgmgr create peer status %d: %s", resp.StatusCode, string(raw))
	}

	var out CreatePeerResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return CreatePeerResponse{}, err
	}
	return out, nil
}
