//go:build !windows

package vm

import (
	"os/exec"
	"syscall"
)

func prepareVMCmd(cmd *exec.Cmd) {
	// This is to prevent Ctrl+C propagating to the child process.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func terminateProcess(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}
