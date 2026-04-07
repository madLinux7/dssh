package sshconfig

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/madLinux7/dssh/internal/model"
)

// remoteCommandRe extracts the directory from a RemoteCommand like:
//
//	cd '/some/path' && exec $SHELL -l
//	cd /some/path && exec $SHELL -l
var remoteCommandRe = regexp.MustCompile(`^cd\s+'?([^']+?)'?\s+&&\s+exec\s+\$SHELL`)

// List parses the ssh_config file at path and returns all non-wildcard Host entries.
func List(path string) ([]model.Connection, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, path)
		}
		return nil, err
	}
	defer f.Close()

	var (
		conns   []model.Connection
		current *model.Connection
		scanner = bufio.NewScanner(f)
	)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// New Host block
		if strings.HasPrefix(trimmed, "Host ") {
			// Flush previous
			if current != nil {
				conns = append(conns, *current)
			}

			hostValue := strings.TrimSpace(strings.TrimPrefix(trimmed, "Host "))

			// Skip wildcard/pattern hosts
			if strings.ContainsAny(hostValue, "*?") {
				current = nil
				continue
			}

			current = &model.Connection{
				Name:     hostValue,
				User:     "root",
				Port:     22,
				AuthType: model.AuthKey,
				Source:   model.SourceSSHConfig,
			}
			continue
		}

		// Skip lines outside a tracked host block
		if current == nil {
			continue
		}

		// Parse key-value pairs inside a Host block
		key, value := parseKeyValue(trimmed)
		if key == "" {
			continue
		}

		switch key {
		case "hostname":
			current.Host = value
		case "user":
			current.User = value
		case "port":
			if p, err := strconv.Atoi(value); err == nil && p > 0 {
				current.Port = p
			}
		case "identityfile":
			current.IdentityFile = value
		case "remotecommand":
			if m := remoteCommandRe.FindStringSubmatch(value); m != nil {
				current.Directory = m[1]
			}
		}
	}

	// Flush last block
	if current != nil {
		conns = append(conns, *current)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return conns, nil
}

// GetByName parses the file and returns the connection with the given name.
func GetByName(path string, name string) (*model.Connection, error) {
	conns, err := List(path)
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(name)
	for i := range conns {
		if strings.ToLower(conns[i].Name) == lower {
			return &conns[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
}

// Exists checks whether a host name exists in the ssh_config file.
func Exists(path string, name string) (bool, error) {
	_, err := GetByName(path, name)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrNotFound.Error())
}

// parseKeyValue splits "    Key Value" into (lowercase_key, value).
func parseKeyValue(line string) (string, string) {
	if line == "" || strings.HasPrefix(line, "#") {
		return "", ""
	}
	// ssh_config allows = or whitespace as separator
	line = strings.ReplaceAll(line, "=", " ")
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.ToLower(strings.TrimSpace(parts[0])), strings.TrimSpace(parts[1])
}
