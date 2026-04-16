package db

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
)

// Config contains Postgres connection settings loaded from environment variables.
//
// Env vars:
// - DB_HOST (default: localhost)
// - DB_PORT (default: 5432)
// - DB_USER (default: postgres)
// - DB_PASSWORD (default: postgres)
// - DB_NAME (default: mira_vpn)
// - DB_SSLMODE (default: disable)
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func DefaultConfig() Config {
	port, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	return Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     port,
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		DBName:   getEnv("DB_NAME", "mira_vpn"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}
}

func (c Config) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   c.DBName,
	}
	q := u.Query()
	q.Set("sslmode", c.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

