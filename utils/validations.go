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
	"encoding/binary"
	"regexp"
	"strings"
	"unicode"

	"github.com/acarl005/stripansi"
)

func ClearUnprintableChars(s string, allowNewlines bool) string {
	// This will remove ANSI color codes.
	s = stripansi.Strip(s)

	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) || (allowNewlines && r == '\n') {
			return r
		}
		return -1
	}, s)
}

var fsTypeRegexp = regexp.MustCompile(`^[a-z0-9]+$`)

func ValidateFsType(s string) bool {
	return fsTypeRegexp.MatchString(s)
}

var mountOptionsRegexp = regexp.MustCompile(`^([a-zA-Z0-9_]+(=[a-zA-Z0-9]+)?)(,[a-zA-Z0-9_]+(=[a-zA-Z0-9]+)?)*$`)

func ValidateMountOptions(s string) bool {
	return mountOptionsRegexp.MatchString(s)
}

var devNameRegexp = regexp.MustCompile(`^[0-9A-Za-z_-]+$`)

func ValidateDevName(s string) bool {
	// Allow mapped devices.
	s = strings.TrimPrefix(s, "mapper/")

	return devNameRegexp.MatchString(s)
}

var unixUsernameRegexp = regexp.MustCompile(`^[a-z_]([a-z0-9_-]{0,31}|[a-z0-9_-]{0,30}\$)$`)

func ValidateUnixUsername(s string) bool {
	return unixUsernameRegexp.MatchString(s)
}

func Uint16ToBytesBE(v uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	return b
}
