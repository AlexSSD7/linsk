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

//go:build darwin

package osspecifics

import (
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func getDeviceLogicalBlockSizeInner(fd uintptr) (int64, error) {
	bs, err := unix.IoctlGetInt(int(fd), 0x40046418) // Syscall code for DKIOGETBLOCKSIZE
	if err != nil {
		return 0, errors.Wrap(err, "ioctl get logical block size")
	}

	return int64(bs), nil
}
