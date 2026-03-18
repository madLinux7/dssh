//go:build !windows

package ssh

import (
	"os"
	"os/exec"
	"syscall"
)

// execSSH replaces the current process with ssh (Unix only).
func execSSH(sshPath string, args []string) error {
	return syscall.Exec(sshPath, args, os.Environ())
}

// setSysProcAttr sets Setsid to detach from controlling terminal for SSH_ASKPASS.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
