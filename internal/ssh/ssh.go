// Package ssh handles launching SSH connections by building command arguments
// and executing the ssh binary.
//
// Two auth modes are supported:
//   - Key auth: replaces the current process with ssh via syscall.Exec (Unix)
//     or runs it as a child process (Windows).
//   - Password auth: creates a temporary askpass script that echoes the password,
//     then runs ssh as a child process with SSH_ASKPASS + SSH_ASKPASS_REQUIRE=force.
//     This avoids a dependency on sshpass and works cross-platform.
package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/madLinux7/dssh/internal/model"
)

// ConnectWithKey replaces the current process with ssh using key-based auth.
func ConnectWithKey(conn *model.Connection, extraArgs []string) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	args := buildArgs(conn, extraArgs)
	return execSSH(sshPath, args)
}

// ConnectWithPassword runs ssh as a child process using SSH_ASKPASS to supply the password.
func ConnectWithPassword(conn *model.Connection, password string, extraArgs []string) error {
	askpassFile, err := writeAskpassScript(password)
	if err != nil {
		return err
	}
	defer os.Remove(askpassFile)

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	args := buildArgs(conn, extraArgs)

	env := append(os.Environ(),
		"SSH_ASKPASS="+askpassFile,
		"SSH_ASKPASS_REQUIRE=force",
	)

	// Remove DISPLAY to avoid conflicts on Linux; SSH_ASKPASS_REQUIRE=force handles it.
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if len(e) > 8 && e[:8] == "DISPLAY=" {
			continue
		}
		filtered = append(filtered, e)
	}

	cmd := exec.Command(sshPath, args[1:]...) // args[0] is "ssh"
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = filtered

	setSysProcAttr(cmd)

	return cmd.Run()
}

// buildArgs constructs the ssh command-line arguments.
// For password auth, it forces password-only authentication via SSH options
// to prevent ssh from trying key-based auth first.
func buildArgs(conn *model.Connection, extraArgs []string) []string {
	args := []string{"ssh"}

	if conn.Directory != "" {
		args = append(args, "-t")
	}

	if conn.Port != 22 {
		args = append(args, "-p", strconv.Itoa(conn.Port))
	}

	if conn.AuthType == model.AuthKey && conn.IdentityFile != "" {
		args = append(args, "-i", conn.IdentityFile)
	}

	if conn.AuthType == model.AuthPassword {
		args = append(args, "-o", "PreferredAuthentications=password",
			"-o", "PubkeyAuthentication=no")
	}

	args = append(args, extraArgs...)
	args = append(args, conn.SSHTarget())

	if conn.Directory != "" {
		remoteCmd := fmt.Sprintf("cd '%s' && exec $SHELL -l",
			escapeShellSingleQuote(conn.Directory))
		args = append(args, remoteCmd)
	}

	return args
}

// writeAskpassScript creates a temporary script that echoes the SSH password.
// SSH calls this script via SSH_ASKPASS when it needs the password.
// The script is deleted by the caller after ssh exits.
func writeAskpassScript(password string) (string, error) {
	tmpDir := os.TempDir()
	var name, content string

	if runtime.GOOS == "windows" {
		name = "dssh-askpass-*.bat"
		content = fmt.Sprintf("@echo off\r\necho %s\r\n", escapeBatchSpecial(password))
	} else {
		name = "dssh-askpass-*.sh"
		content = fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '%s'\n", escapeShellSingleQuote(password))
	}

	f, err := os.CreateTemp(tmpDir, name)
	if err != nil {
		return "", fmt.Errorf("create askpass script: %w", err)
	}

	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("write askpass script: %w", err)
	}
	f.Close()

	if runtime.GOOS != "windows" {
		if err := os.Chmod(f.Name(), 0700); err != nil {
			os.Remove(f.Name())
			return "", fmt.Errorf("chmod askpass script: %w", err)
		}
	}

	abs, err := filepath.Abs(f.Name())
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return abs, nil
}

// escapeBatchSpecial escapes characters that have special meaning in batch scripts (& | < > ^ %) to prevent command injection.
// Big Thanks to Cody Churchwell (https://github.com/consigcody94) for pointing this out!
func escapeBatchSpecial(s string) string {
	result := make([]byte, 0, len(s)+10)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '^', '&', '|', '<', '>', '(', ')', '!':
			result = append(result, '^', s[i])
		case '%':
			result = append(result, '%', '%')
		default:
			result = append(result, s[i])
		}
	}
	return string(result)
}

// escapeShellSingleQuote escapes single quotes for safe embedding in a
// single-quoted shell string. Each ' becomes '\'' (end quote, escaped quote, start quote).
func escapeShellSingleQuote(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			result = append(result, '\'', '\\', '\'', '\'')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}
