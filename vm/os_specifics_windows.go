// go:build windows

package vm

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/pkg/errors"
)

func prepareVMCmd(cmd *exec.Cmd) {
	// This is to prevent Ctrl+C propagating to the child process.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func terminateProcess(pid int) error {
	return exec.Command("TASKKILL", "/T", "/F", "/PID", fmt.Sprint(pid)).Run()
}

var physicalDriveRegexp = regexp.MustCompile(`PhysicalDrive(\d+)`)

// This is never used except for a band-aid that would check
// that there are no double-mounts.
func checkDeviceSeemsMounted(path string) (bool, error) {
	// Quite a bit hacky implementation, but it's to be used as a failsafe band-aid anyway.
	matches := physicalDriveRegexp.FindAllStringSubmatch(path, 1)
	if len(matches) == 0 {
		return false, fmt.Errorf("bad device path '%v'", path)
	}

	match := matches[0]

	if want, have := 2, len(match); want != have {
		return false, fmt.Errorf("bad match items length: want %v, have %v (%v)", want, have, match)
	}

	out, err := exec.Command("wmic", "path", "Win32_LogicalDiskToPartition", "get", "Antecedent").Output()
	if err != nil {
		return false, errors.Wrap(err, "exec wmic cmd")
	}

	return strings.Contains(string(out), fmt.Sprintf("Disk #%v", match[1])), nil
}
