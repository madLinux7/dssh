package sshconfig

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// EnsureIncludeDirective checks ~/.ssh/config for an Include config.d/* directive.
// If absent, it prepends one. It also creates ~/.ssh/config.d/ (0700) and the
// dssh file (0600) if they don't exist.
func EnsureIncludeDirective() error {
	mainPath, err := MainFilePath()
	if err != nil {
		return err
	}

	// Create ~/.ssh/config if it doesn't exist
	if err := CreateMainFile(); err != nil {
		return err
	}

	// Check if Include config.d/* already present
	hasInclude, err := hasIncludeDirective(mainPath)
	if err != nil {
		return err
	}

	if !hasInclude {
		if err := prependIncludeDirective(mainPath); err != nil {
			return err
		}
	}

	// Create directive dir and file
	return CreateDirectiveFile()
}

// CreateMainFile creates ~/.ssh/config with 0600 permissions if it doesn't exist.
// It also ensures ~/.ssh/ directory exists.
func CreateMainFile() error {
	mainPath, err := MainFilePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(mainPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	if _, err := os.Stat(mainPath); os.IsNotExist(err) {
		return os.WriteFile(mainPath, []byte(""), 0600)
	}
	return nil
}

// CreateDirectiveFile creates ~/.ssh/config.d/dssh (and its parent dir) if they
// don't exist.
func CreateDirectiveFile() error {
	dirPath, err := DirectiveFilePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(dirPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return os.WriteFile(dirPath, []byte(""), 0600)
	}
	return nil
}

// hasIncludeDirective scans the file for a line like "Include config.d/*".
func hasIncludeDirective(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "include") {
			// Check if it references config.d
			rest := strings.TrimSpace(trimmed[len("include"):])
			rest = strings.TrimLeft(rest, "= ")
			if strings.Contains(rest, "config.d/") || strings.Contains(rest, "config.d/*") {
				return true, nil
			}
		}
	}
	return false, scanner.Err()
}

// prependIncludeDirective adds "Include config.d/*" at the top of the file.
func prependIncludeDirective(path string) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	directive := "Include config.d/*\n\n"
	content := directive + string(existing)
	return os.WriteFile(path, []byte(content), 0600)
}
