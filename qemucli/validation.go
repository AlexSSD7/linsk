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

package qemucli

import (
	"fmt"
	"strings"
)

func validateArgKey(key string, t ArgAcceptedValue) error {
	allowedValue, ok := safeArgs[key]
	if !ok {
		return fmt.Errorf("unknown safe arg '%v'", key)
	}

	if want, have := allowedValue, t; want != have {
		return fmt.Errorf("bad arg value type: want '%v', have '%v'", allowedValue, t)
	}

	return nil
}

func validateArgStrValue(s string) error {
	if strings.Contains(s, ",") {
		return fmt.Errorf("commas are not allowed")
	}

	if strings.Contains(s, "\\") {
		// Backslashes are theoretically allowed, but they rarely work as intended.
		// For Windows paths, forward slashes should be used.
		return fmt.Errorf("backslashes are not allowed")
	}

	if strings.Contains(s, "=") {
		return fmt.Errorf("equals sign is not allowed")
	}

	return nil
}
