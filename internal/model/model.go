package model

import "fmt"

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
