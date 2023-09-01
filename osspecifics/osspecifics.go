//go:build !windows

package osspecifics

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"
)

func SetNewProcessGroupCmd(cmd *exec.Cmd) {
	// This is to prevent Ctrl+C propagating to the child process.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func TerminateProcess(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

// This is never used except for a band-aid that would check
// that there are no double-mounts.
func CheckDeviceSeemsMounted(devPathPrefix string) (bool, error) {
	// Quite a bit hacky implementation, but it's to be used as a failsafe band-aid anyway.
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

func CheckValidDevicePath(devPath string) error {
	stat, err := os.Stat(devPath)
	if err != nil {
		return errors.Wrap(err, "stat path")
	}

	isDev := stat.Mode()&os.ModeDevice != 0
	if !isDev {
		return fmt.Errorf("file mode is not device (%v)", stat.Mode())
	}

	return nil
}

func CheckRunAsRoot() (bool, error) {
	currentUser, err := user.Current()
	if err != nil {
		return false, errors.Wrap(err, "get current user")
	}

	return currentUser.Username == "root", nil
}
