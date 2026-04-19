// Package db manages the SQLite database at ~/.dssh/dssh.db.
//
// Uses a pure-Go SQLite driver (modernc.org/sqlite) so the binary can be
// built with CGO_ENABLED=0 for fully static, cross-platform binaries.
// Two tables: connections (SSH connection configs) and settings (key-value store
// for crypto salt and passphrase verification token).
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS connections (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL UNIQUE,
    user            TEXT    NOT NULL,
    host            TEXT    NOT NULL,
    port            INTEGER NOT NULL DEFAULT 22,
    directory       TEXT    NOT NULL DEFAULT '',
    auth_type       TEXT    NOT NULL DEFAULT 'key',
    identity_file   TEXT    NOT NULL DEFAULT '',
    proxy_jump      TEXT    NOT NULL DEFAULT '',
    encrypted_pass  BLOB    NOT NULL DEFAULT x'',
    pass_nonce      BLOB    NOT NULL DEFAULT x'',
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value BLOB NOT NULL
);
`

// Open opens or creates the SQLite database at ~/.dssh/dssh.db and runs migrations.
func Open() (*sql.DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	dir := filepath.Join(home, ".dssh")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dir, "dssh.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent access.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// migrate applies idempotent column additions for databases created by older
// dssh versions. Each step is guarded by a PRAGMA check so the migration can
// be run on any database without losing data.
func migrate(db *sql.DB) error {
	return addColumnIfMissing(db, "connections", "proxy_jump", "TEXT NOT NULL DEFAULT ''")
}

// addColumnIfMissing adds a column to a table only if it is not already present.
func addColumnIfMissing(db *sql.DB, table, column, columnDef string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("inspect %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid           int
			name, ctype   string
			notnull, pk   int
			dfltValue     any
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("scan %s column: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, columnDef)
	if _, err := db.Exec(stmt); err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}
