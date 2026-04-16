package db

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

// Open returns a connected Postgres connection.
// It pings the DB so callers fail fast during local development/CI.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

