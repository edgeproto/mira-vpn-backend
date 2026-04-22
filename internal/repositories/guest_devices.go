package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type GuestDevicesRepository struct {
	db *sql.DB
}

func NewGuestDevicesRepository(db *sql.DB) *GuestDevicesRepository {
	return &GuestDevicesRepository{db: db}
}

// ResolveUserID returns a stable user id for a device id, creating a guest user on first use.
func (r *GuestDevicesRepository) ResolveUserID(ctx context.Context, deviceID string) (string, error) {
	email := fmt.Sprintf("guest_%s@mira.local", sanitizeDeviceID(deviceID))
	var userID string
	err := r.db.QueryRowContext(
		ctx,
		`WITH upsert_user AS (
		   INSERT INTO users (email, password_hash, is_pro)
		   VALUES ($2, '', false)
		   ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		   RETURNING id
		 ),
		 upsert_guest AS (
		   INSERT INTO guest_devices (device_id, user_id)
		   VALUES ($1, (SELECT id FROM upsert_user))
		   ON CONFLICT (device_id) DO UPDATE SET user_id = guest_devices.user_id
		   RETURNING user_id
		 )
		 SELECT user_id::text FROM upsert_guest`,
		deviceID,
		email,
	).Scan(&userID)
	if err != nil {
		return "", err
	}
	return userID, nil
}

func sanitizeDeviceID(deviceID string) string {
	raw := strings.ToLower(strings.TrimSpace(deviceID))
	if raw == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('_')
	}
	return b.String()
}
