package models

import "time"

// User represents an authenticated account in the system.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	IsPro        bool      `json:"isPro"`
	CreatedAt    time.Time `json:"createdAt"`
}
