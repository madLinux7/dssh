package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// GetSetting retrieves a binary value by key. Returns nil if not found.
func GetSetting(db *sql.DB, key string) ([]byte, error) {
	var val []byte
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get setting %q: %w", key, err)
	}
	return val, nil
}

// SetSetting upserts a key-value pair in the settings table.
func SetSetting(db *sql.DB, key string, value []byte) error {
	_, err := db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}
