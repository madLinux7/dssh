//go:build windows

package ssh

import (
	"os"
	"os/exec"
)

// execSSH runs ssh as a child process on Windows (no syscall.Exec).
func execSSH(sshPath string, args []string) error {
	cmd := exec.Command(sshPath, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// setSysProcAttr is a no-op on Windows (no Setsid).
func setSysProcAttr(cmd *exec.Cmd) {}
