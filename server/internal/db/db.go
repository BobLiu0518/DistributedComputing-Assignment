package db

import (
	"crypto/sha256"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

var driverName = "sqlite"

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return nil, fmt.Errorf("create users table: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS cards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			stars INTEGER NOT NULL,
			drawn_at DATETIME DEFAULT CURRENT_TIMESTAMP
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
