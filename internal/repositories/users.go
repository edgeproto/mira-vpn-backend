package repositories

import (
	"context"
	"database/sql"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/models"
)

// UsersRepository provides Postgres-backed persistence for users.
type UsersRepository struct {
	db *sql.DB
}

func NewUsersRepository(db *sql.DB) *UsersRepository {
	return &UsersRepository{db: db}
}

func (r *UsersRepository) Create(
	ctx context.Context,
	email string,
	passwordHash string,
	isPro bool,
) (models.User, error) {
	var u models.User
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO users (email, password_hash, is_pro)
		 VALUES ($1, $2, $3)
		 RETURNING id::text, email, password_hash, is_pro, created_at`,
		email,
		passwordHash,
		isPro,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsPro, &u.CreatedAt)
	return u, err
}

func (r *UsersRepository) GetByEmail(ctx context.Context, email string) (models.User, error) {
	var u models.User
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id::text, email, password_hash, is_pro, created_at
		 FROM users
		 WHERE email = $1
		 LIMIT 1`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsPro, &u.CreatedAt)

	if err == sql.ErrNoRows {
		return models.User{}, ErrNotFound
	}
	return u, err
}

func (r *UsersRepository) GetByID(ctx context.Context, id string) (models.User, error) {
	var u models.User
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id::text, email, password_hash, is_pro, created_at
		 FROM users
		 WHERE id = $1::uuid
		 LIMIT 1`,
		id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsPro, &u.CreatedAt)

	if err == sql.ErrNoRows {
		return models.User{}, ErrNotFound
	}
	return u, err
}
