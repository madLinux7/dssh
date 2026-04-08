package sshconfig

import (
	"fmt"
	"os"
	"strings"

	"github.com/madLinux7/dssh/internal/model"
)

// Insert appends a new Host block after the last non-wildcard entry in the file.
// If the file is empty or has no individual hosts, it appends at the end.
func Insert(path string, conn *model.Connection) error {
	lines, err := readLines(path)
	if err != nil {
		if isConfigNotFound(err) {
			// File doesn't exist yet — write just the block
			return os.WriteFile(path, []byte(formatBlock(conn)+"\n"), 0600)
		}
		return err
	}

	// Check for duplicate name
	exists, err := Exists(path, conn.Name)
	if err != nil && !isConfigNotFound(err) {
		return err
	}
	if exists {
		return fmt.Errorf("%w: %q", ErrDuplicateName, conn.Name)
	}

	insertIdx := findLastIndividualHostEnd(lines)
	block := formatBlock(conn)

	var result []string
	if insertIdx == len(lines) {
		// Append at end
		result = append(lines, "")
		result = append(result, strings.Split(block, "\n")...)
	} else {
		result = make([]string, 0, len(lines)+10)
		result = append(result, lines[:insertIdx]...)
		result = append(result, "")
		result = append(result, strings.Split(block, "\n")...)
		result = append(result, lines[insertIdx:]...)
	}

	return writeLines(path, result)
}

// Update replaces the Host block with the given oldName with the new connection data.
func Update(path string, oldName string, conn *model.Connection) error {
	lines, err := readLines(path)
	if err != nil {
		return err
	}

	start, end, found := findBlockBounds(lines, oldName)
	if !found {
		return fmt.Errorf("%w: %q in %s — was the file edited manually?", ErrNotFound, oldName, path)
	}

	// If name changed, check the new name doesn't already exist
	if !strings.EqualFold(oldName, conn.Name) {
		exists, err := Exists(path, conn.Name)
		if err != nil && !isConfigNotFound(err) {
			return err
		}
		if exists {
			return fmt.Errorf("%w: %q", ErrDuplicateName, conn.Name)
		}
	}

	block := formatBlock(conn)
	newBlockLines := strings.Split(block, "\n")

	result := make([]string, 0, len(lines))
	result = append(result, lines[:start]...)
	result = append(result, newBlockLines...)
	result = append(result, lines[end:]...)

	return writeLines(path, result)
}

// Delete removes the Host block with the given name from the file.
func Delete(path string, name string) error {
	lines, err := readLines(path)
	if err != nil {
		return err
	}

	start, end, found := findBlockBounds(lines, name)
	if !found {
		return fmt.Errorf("%w: %q in %s — was the file edited manually?", ErrNotFound, name, path)
	}

	// Also consume one trailing blank line if present
	if end < len(lines) && strings.TrimSpace(lines[end]) == "" {
		end++
	}

	result := make([]string, 0, len(lines))
	result = append(result, lines[:start]...)
	result = append(result, lines[end:]...)

	// Remove leading blank line if block was at the start
	if start == 0 && len(result) > 0 && strings.TrimSpace(result[0]) == "" {
		result = result[1:]
	}

	return writeLines(path, result)
}

// formatBlock produces the ssh_config text for a connection (no trailing newline).
func formatBlock(conn *model.Connection) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Host %s\n", conn.Name)
	fmt.Fprintf(&b, "    HostName %s\n", conn.Host)
	fmt.Fprintf(&b, "    User %s\n", conn.User)
	if conn.Port != 22 {
		fmt.Fprintf(&b, "    Port %d\n", conn.Port)
	}
	if conn.IdentityFile != "" && conn.IdentityFile != "default" {
		fmt.Fprintf(&b, "    IdentityFile %s\n", conn.IdentityFile)
	}
	if conn.Directory != "" {
		fmt.Fprintf(&b, "    RemoteCommand cd '%s' && exec $SHELL -l\n", conn.Directory)
		fmt.Fprintf(&b, "    RequestTTY force\n")
	}
	// Trim final newline so callers can control spacing
	return strings.TrimRight(b.String(), "\n")
}

// findBlockBounds locates the start and end (exclusive) line indices for a Host block.
func findBlockBounds(lines []string, name string) (start, end int, found bool) {
	lower := strings.ToLower(name)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Host ") {
			hostValue := strings.TrimSpace(strings.TrimPrefix(trimmed, "Host "))
			if strings.ToLower(hostValue) == lower {
				start = i
				found = true
				// Scan forward to find the end of the block
				end = len(lines)
				for j := i + 1; j < len(lines); j++ {
					jTrimmed := strings.TrimSpace(lines[j])
					if strings.HasPrefix(jTrimmed, "Host ") {
						end = j
						break
					}
				}
				// Trim trailing blank lines within the block
				for end > start+1 && strings.TrimSpace(lines[end-1]) == "" {
					end--
				}
				return
			}
		}
	}
	return 0, 0, false
}

// findLastIndividualHostEnd returns the line index after the last non-wildcard Host block.
func findLastIndividualHostEnd(lines []string) int {
	lastEnd := len(lines)
	foundAny := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Host ") {
			continue
		}
		hostValue := strings.TrimSpace(strings.TrimPrefix(trimmed, "Host "))
		if strings.ContainsAny(hostValue, "*?") {
			continue
		}
		foundAny = true
		// Find end of this block
		blockEnd := len(lines)
		for j := i + 1; j < len(lines); j++ {
			jTrimmed := strings.TrimSpace(lines[j])
			if strings.HasPrefix(jTrimmed, "Host ") {
				blockEnd = j
				break
			}
		}
		// Trim trailing blank lines
		for blockEnd > i+1 && strings.TrimSpace(lines[blockEnd-1]) == "" {
			blockEnd--
		}
		lastEnd = blockEnd
	}

	if !foundAny {
		return len(lines)
	}
	return lastEnd
}

// readLines reads a file into a slice of lines.
func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s — Use 'dssh config' to reconfigure", ErrConfigNotFound, path)
		}
		return nil, err
	}
	content := string(data)
	if content == "" {
		return nil, nil
	}
	// Remove trailing newline to avoid a phantom empty line
	content = strings.TrimRight(content, "\n")
	return strings.Split(content, "\n"), nil
}

// writeLines writes lines back to a file, with a trailing newline.
func writeLines(path string, lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0600)
}

func isConfigNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrConfigNotFound.Error())
}
