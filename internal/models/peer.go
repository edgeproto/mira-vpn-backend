package models

import "time"

// Peer represents a WireGuard peer associated with a user.
type Peer struct {
	ID          string    `json:"id"`
	UserID      string    `json:"userId"`
	Location    string    `json:"location"`
	WgPublicKey string    `json:"wgPublicKey"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}
