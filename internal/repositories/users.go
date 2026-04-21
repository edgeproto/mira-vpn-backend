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

func (r *UsersRepository) GetByProviderSubject(ctx context.Context, provider string, subject string) (models.User, error) {
	var u models.User
	err := r.db.QueryRowContext(
		ctx,
		`SELECT u.id::text, u.email, u.password_hash, u.is_pro, u.created_at
		 FROM users u
		 INNER JOIN auth_identities ai ON ai.user_id = u.id
		 WHERE ai.provider = $1 AND ai.provider_subject = $2
		 LIMIT 1`,
		provider,
		subject,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsPro, &u.CreatedAt)

	if err == sql.ErrNoRows {
		return models.User{}, ErrNotFound
	}
	return u, err
}

func (r *UsersRepository) CreateIdentity(
	ctx context.Context,
	userID string,
	provider string,
	providerSubject string,
	email string,
) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO auth_identities (user_id, provider, provider_subject, email)
		 VALUES ($1::uuid, $2, $3, $4)
		 ON CONFLICT (provider, provider_subject) DO NOTHING`,
		userID,
		provider,
		providerSubject,
		email,
	)
	return err
}
