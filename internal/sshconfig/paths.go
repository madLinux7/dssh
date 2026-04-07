package sshconfig

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/madLinux7/dssh/internal/model"
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

// FilePath returns the ssh_config path for the given target.
func FilePath(target model.SSHConfigTarget) (string, error) {
	switch target {
	case model.SSHConfigTargetDirective:
		return DirectiveFilePath()
	default:
		return MainFilePath()
	}
}

// FileExists checks whether the ssh_config file for the given target exists.
func FileExists(target model.SSHConfigTarget) bool {
	p, err := FilePath(target)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}
