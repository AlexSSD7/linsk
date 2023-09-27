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

package osspecifics

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
)

func GetDeviceLogicalBlockSize(devPath string) (uint64, error) {
	fd, err := os.Open(devPath)
	if err != nil {
		return 0, errors.Wrap(err, "open device")
	}

	defer func() { _ = fd.Close() }()

	bs, err := getDeviceLogicalBlockSizeInner(fd.Fd())
	if err != nil {
		return 0, errors.Wrap(err, "get block size inner")
	}

	if bs <= 0 {
		return 0, fmt.Errorf("retrieved block size is zero (or negative): '%v'", bs)
	}

	return uint64(bs), nil
}
