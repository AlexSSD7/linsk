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

//go:build linux

package osspecifics

import (
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func getDeviceLogicalBlockSizeInner(fd uintptr) (int64, error) {
	var bs int64
	_, _, serr := unix.Syscall(unix.SYS_IOCTL, fd, unix.BLKSSZGET, uintptr(unsafe.Pointer(&bs))) // #nosec G103 It's safe.
	if serr != 0 {
		return 0, errors.Wrap(serr, "syscall get logical block size")
	}

	return bs, nil
}
