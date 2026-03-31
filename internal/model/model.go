package model

import (
	"fmt"
	"strings"
)

// ReservedNames are CLI subcommand names that cannot be used as connection names.
var ReservedNames = map[string]bool{
	"add":    true,
	"rm":     true,
	"list":   true,
	"new":    true,
	"create": true,
	"edit":   true,
	"delete": true,
	"reset":  true,
	"help":   true,
}

// ErrReservedName is returned when a connection name conflicts with a CLI command.
var ErrReservedName = fmt.Errorf("name is a reserved command")

// ValidateName checks whether a connection name conflicts with a CLI command.
func ValidateName(name string) error {
	if ReservedNames[strings.ToLower(name)] {
		return fmt.Errorf("%w: %q", ErrReservedName, name)
	}
	return nil
}

// AuthType represents the SSH authentication method.
type AuthType string

const (
	AuthKey      AuthType = "key"
	AuthPassword AuthType = "password"
)

// Connection represents a saved SSH connection.
type Connection struct {
	ID            int64
	Name          string
	User          string
	Host          string
	Port          int
	Directory     string
	AuthType      AuthType
	IdentityFile  string
	EncryptedPass []byte
	PassNonce     []byte
	CreatedAt     string
	UpdatedAt     string
}

// SSHTarget returns user@host.
func (c Connection) SSHTarget() string {
	return fmt.Sprintf("%s@%s", c.User, c.Host)
}

// DisplayLabel returns a formatted label for TUI display.
func (c Connection) DisplayLabel() string {
	if c.Port != 22 {
		return fmt.Sprintf("%s — %s@%s:%d", c.Name, c.User, c.Host, c.Port)
	}
	return fmt.Sprintf("%s — %s@%s", c.Name, c.User, c.Host)
}
