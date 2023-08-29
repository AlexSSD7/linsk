//go:build !windows

package vm

import (
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"
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

// This is never used except for a band-aid that would check
// that there are no double-mounts.
func checkDeviceSeemsMounted(devPathPrefix string) (bool, error) {
	absDevPathPrefix, err := filepath.Abs(devPathPrefix)
	if err != nil {
		return false, errors.Wrap(err, "get abs path")
	}

	mounts, err := exec.Command("mount").Output()
	if err != nil {
		return false, errors.Wrap(err, "run mount command")
	}

	for _, line := range strings.Split(string(mounts), "\n") {
		// I know, I know, this is a rare band-aid.
		if strings.HasPrefix(line, devPathPrefix) || strings.HasPrefix(line, absDevPathPrefix) {
			return true, nil
		}
	}

	return false, nil
}
