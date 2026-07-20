package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/madLinux7/dssh/internal/model"
)

// ErrDuplicateName is returned when a connection name already exists.
var ErrDuplicateName = errors.New("a connection with that name already exists")

// ErrNotFound is returned when a connection is not found.
var ErrNotFound = errors.New("connection not found")

// Insert adds a new connection to the database.
func Insert(db *sql.DB, c *model.Connection) error {
	return insertConnection(db, c)
}

type connectionExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func insertConnection(execer connectionExecer, c *model.Connection) error {
	encPass := c.EncryptedPass
	if encPass == nil {
		encPass = []byte{}
	}
	nonce := c.PassNonce
	if nonce == nil {
		nonce = []byte{}
	}
	result, err := execer.Exec(`
		INSERT INTO connections (name, user, host, port, directory, auth_type, identity_file, proxy_jump, encrypted_pass, pass_nonce)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.User, c.Host, c.Port, c.Directory, string(c.AuthType), c.IdentityFile, c.ProxyJump, encPass, nonce,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("%w: %q", ErrDuplicateName, c.Name)
		}
		return fmt.Errorf("insert connection: %w", err)
	}
	if id, idErr := result.LastInsertId(); idErr == nil {
		c.ID = id
	}
	return nil
}

// GetByName retrieves a connection by its unique name.
func GetByName(db *sql.DB, name string) (*model.Connection, error) {
	c := &model.Connection{}
	err := db.QueryRow(`
		SELECT id, name, user, host, port, directory, auth_type, identity_file, proxy_jump, encrypted_pass, pass_nonce, created_at, updated_at
		FROM connections WHERE name = ?`, name,
	).Scan(&c.ID, &c.Name, &c.User, &c.Host, &c.Port, &c.Directory, &c.AuthType, &c.IdentityFile, &c.ProxyJump, &c.EncryptedPass, &c.PassNonce, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	if err != nil {
		return nil, fmt.Errorf("get connection %q: %w", name, err)
	}
	return c, nil
}

// List returns all connections ordered by name.
func List(db *sql.DB) ([]model.Connection, error) {
	rows, err := db.Query(`
		SELECT id, name, user, host, port, directory, auth_type, identity_file, proxy_jump, encrypted_pass, pass_nonce, created_at, updated_at
		FROM connections ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer rows.Close()

	var conns []model.Connection
	for rows.Next() {
		var c model.Connection
		if err := rows.Scan(&c.ID, &c.Name, &c.User, &c.Host, &c.Port, &c.Directory, &c.AuthType, &c.IdentityFile, &c.ProxyJump, &c.EncryptedPass, &c.PassNonce, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan connection: %w", err)
		}
		conns = append(conns, c)
	}
	return conns, rows.Err()
}

// Update modifies an existing connection identified by ID.
func Update(db *sql.DB, c *model.Connection) error {
	return updateConnection(db, c)
}

func updateConnection(execer connectionExecer, c *model.Connection) error {
	encPass := c.EncryptedPass
	if encPass == nil {
		encPass = []byte{}
	}
	nonce := c.PassNonce
	if nonce == nil {
		nonce = []byte{}
	}
	result, err := execer.Exec(`
		UPDATE connections
		SET name=?, user=?, host=?, port=?, directory=?, auth_type=?,
		    identity_file=?, proxy_jump=?, encrypted_pass=?, pass_nonce=?, updated_at=datetime('now')
		WHERE id=?`,
		c.Name, c.User, c.Host, c.Port, c.Directory, string(c.AuthType),
		c.IdentityFile, c.ProxyJump, encPass, nonce, c.ID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("%w: %q", ErrDuplicateName, c.Name)
		}
		return fmt.Errorf("update connection: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, c.Name)
	}
	return nil
}

// Delete removes a connection by name.
func Delete(db *sql.DB, name string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete connection: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`
		DELETE FROM connection_group_memberships
		WHERE source = ? AND source_path = '' AND connection_name = ?`, model.SourceSQLite, name); err != nil {
		return fmt.Errorf("delete connection memberships: %w", err)
	}
	res, err := tx.Exec("DELETE FROM connections WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete connection: %w", err)
	}
	return nil
}
