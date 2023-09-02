// Linsk - A utility to access Linux-native file systems on non-Linux operating systems.
// Copyright (c) 2023 The Linsk Authors.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
