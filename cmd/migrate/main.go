package main

import (
	"context"
	"flag"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/wesdod/mira-vpn/mira-vpn-backend/internal/db"
)

func main() {
	var (
		migrationsDir = flag.String("migrations-dir", "internal/db/migrations", "Migrations directory (relative to repo root or current working directory).")
		direction     = flag.String("direction", "up", "Migration direction: up|down.")
		timeout       = flag.Duration("timeout", 30*time.Second, "Overall timeout for applying migrations.")
	)
	flag.Parse()

	cfg := db.DefaultConfig()

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// migrate's "file://" source expects a real filesystem path.
	migrationsPath := filepath.Join(wd, *migrationsDir)
	sourceURL := (&url.URL{Scheme: "file", Path: migrationsPath}).String()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Create DSN once so migrate doesn't need env resolution.
	dsn := cfg.DSN()

	// migrate.New expects a postgres:// DSN (sslmode included).
	m, err := migrate.New(sourceURL, dsn)
	if err != nil {
		log.Fatal(err)
	}

	apply := func() error {
		switch *direction {
		case "up":
			return m.Up()
		case "down":
			return m.Down()
		default:
			return &flagError{msg: "invalid -direction (expected up|down)"}
		}
	}

	// Migrations can take a little while; keep the CLI responsive to timeouts.
	done := make(chan error, 1)
	go func() { done <- apply() }()

	select {
	case <-ctx.Done():
		log.Fatal("migrations timed out")
	case err := <-done:
		if err == nil || err == migrate.ErrNoChange {
			log.Printf("migrations applied (direction=%s)", *direction)
			return
		}
		if err != nil {
			log.Fatal(err)
		}
	}
}

type flagError struct{ msg string }

func (e *flagError) Error() string { return e.msg }

