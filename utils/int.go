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

package utils

import (
	"strconv"

	"golang.org/x/exp/constraints"
)

func IntToStr[T constraints.Signed](v T) string {
	return strconv.FormatInt(int64(v), 10)
}

func UintToStr[T constraints.Unsigned](v T) string {
	return strconv.FormatUint(uint64(v), 10)
}
