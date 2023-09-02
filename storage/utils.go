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

package storage

import (
	"os"

	"github.com/pkg/errors"
)

func checkExistsOrRemove(path string, overwriteRemove bool) (bool, error) {
	var removed bool

	_, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return removed, errors.Wrap(err, "stat file")
		}
	} else {
		if overwriteRemove {
			err = os.Remove(path)
			if err != nil {
				return removed, errors.Wrap(err, "remove file")
			}
			removed = true
		} else {
			return removed, ErrImageAlreadyExists
		}
	}

	return removed, nil
}
