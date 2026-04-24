package repositories

import (
	"context"
	"database/sql"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/models"
)

// PeersRepository provides Postgres-backed persistence for WireGuard peers.
type PeersRepository struct {
	db *sql.DB
}

func NewPeersRepository(db *sql.DB) *PeersRepository {
	return &PeersRepository{db: db}
}

// Upsert inserts or updates the peer row for (user_id, location).
// WireGuard config fetch must be idempotent so clients can refresh after
// server reboots or profile changes without getting HTTP 409.
func (r *PeersRepository) Upsert(
	ctx context.Context,
	userID string,
	location string,
	wgPublicKey string,
	status string,
) (models.Peer, error) {
	if status == "" {
		status = "pending"
	}

	var p models.Peer
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO peers (user_id, location, wg_public_key, status)
		 VALUES ($1::uuid, $2, $3, $4)
		 ON CONFLICT (user_id, location) DO UPDATE
		 SET wg_public_key = EXCLUDED.wg_public_key,
		     status = EXCLUDED.status
		 RETURNING id::text, user_id::text, location, wg_public_key, status, created_at`,
		userID,
		location,
		wgPublicKey,
		status,
	).Scan(&p.ID, &p.UserID, &p.Location, &p.WgPublicKey, &p.Status, &p.CreatedAt)

	return p, err
}

func (r *PeersRepository) Create(
	ctx context.Context,
	userID string,
	location string,
	wgPublicKey string,
	status string,
) (models.Peer, error) {
	if status == "" {
		status = "pending"
	}

	var p models.Peer
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO peers (user_id, location, wg_public_key, status)
		 VALUES ($1::uuid, $2, $3, $4)
		 RETURNING id::text, user_id::text, location, wg_public_key, status, created_at`,
		userID,
		location,
		wgPublicKey,
		status,
	).Scan(&p.ID, &p.UserID, &p.Location, &p.WgPublicKey, &p.Status, &p.CreatedAt)

	return p, err
}

func (r *PeersRepository) GetByUserAndLocation(
	ctx context.Context,
	userID string,
	location string,
) (models.Peer, error) {
	var p models.Peer
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id::text, user_id::text, location, wg_public_key, status, created_at
		 FROM peers
		 WHERE user_id = $1::uuid AND location = $2
		 LIMIT 1`,
		userID,
		location,
	).Scan(&p.ID, &p.UserID, &p.Location, &p.WgPublicKey, &p.Status, &p.CreatedAt)

	if err == sql.ErrNoRows {
		return models.Peer{}, ErrNotFound
	}
	return p, err
}

func (r *PeersRepository) GetByID(ctx context.Context, peerID string) (models.Peer, error) {
	var p models.Peer
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id::text, user_id::text, location, wg_public_key, status, created_at
		 FROM peers
		 WHERE id = $1::uuid
		 LIMIT 1`,
		peerID,
	).Scan(&p.ID, &p.UserID, &p.Location, &p.WgPublicKey, &p.Status, &p.CreatedAt)

	if err == sql.ErrNoRows {
		return models.Peer{}, ErrNotFound
	}
	return p, err
}

func (r *PeersRepository) SetStatus(
	ctx context.Context,
	peerID string,
	status string,
) (models.Peer, error) {
	var p models.Peer
	err := r.db.QueryRowContext(
		ctx,
		`UPDATE peers
		 SET status = $2
		 WHERE id = $1::uuid
		 RETURNING id::text, user_id::text, location, wg_public_key, status, created_at`,
		peerID,
		status,
	).Scan(&p.ID, &p.UserID, &p.Location, &p.WgPublicKey, &p.Status, &p.CreatedAt)

	if err == sql.ErrNoRows {
		return models.Peer{}, ErrNotFound
	}
	return p, err
}
