package db

import (
	"crypto/sha256"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

func Open(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`); err != nil {
		return nil, fmt.Errorf("create users table: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS cards (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			name TEXT NOT NULL,
			stars INTEGER NOT NULL,
			drawn_at TIMESTAMP DEFAULT NOW()
		)
	`); err != nil {
		return nil, fmt.Errorf("create cards table: %w", err)
	}

	return db, nil
}

func HashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", h)
}
