// go:build windows

package osspecifics

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

func SetNewProcessGroupCmd(cmd *exec.Cmd) {
	// This is to prevent Ctrl+C propagating to the child process.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func TerminateProcess(pid int) error {
	return exec.Command("TASKKILL", "/T", "/F", "/PID", fmt.Sprint(pid)).Run()
}

var physicalDriveCheckRegexp = regexp.MustCompile(`^\\\\.\\PhysicalDrive(\d+)$`)
var physicalDriveFindRegexp = regexp.MustCompile(`PhysicalDrive(\d+)`)

// This is never used except for a band-aid that would check
// that there are no double-mounts.
func CheckDeviceSeemsMounted(path string) (bool, error) {
	// Quite a bit hacky implementation, but it's to be used as a failsafe band-aid anyway.
	matches := physicalDriveFindRegexp.FindAllStringSubmatch(path, 1)
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

func CheckValidDevicePath(devPath string) error {
	if !physicalDriveCheckRegexp.MatchString(devPath) {
		// Not including the device path in Errorf() as it is supposed to
		// be included outside when the error is handled.
		return fmt.Errorf("invalid device path")
	}

	return nil
}

func CheckRunAsRoot() (bool, error) {
	var sid *windows.SID

	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		return false, errors.Wrap(err, "allocate and initiliaze win sid")
	}

	defer func() { _ = windows.FreeSid(sid) }()

	member, err := windows.Token(0).IsMember(sid)
	if err != nil {
		return false, errors.Wrap(err, "check win sid membership")
	}

	return member, nil
}
