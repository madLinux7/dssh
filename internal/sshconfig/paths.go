package sshconfig

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	ErrConfigNotFound = errors.New("ssh config file not found")
	ErrDuplicateName  = errors.New("duplicate host name")
	ErrNotFound       = errors.New("host not found")
)

// MainFilePath returns the path to ~/.ssh/config.
func MainFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

// DirectiveFilePath returns the path to ~/.ssh/config.d/dssh.
func DirectiveFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config.d", "dssh"), nil
}
