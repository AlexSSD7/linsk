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
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

func WrapErrWithLog(err error, msg, log string) error {
	return errors.Wrapf(err, "%v %v", msg, GetLogErrMsg(log, "log"))
}

func GetLogErrMsg(s string, logLabel string) string {
	logToInclude := strings.ReplaceAll(s, "\n", "\\n")
	logToInclude = strings.TrimSuffix(logToInclude, "\\n")
	logToInclude = ClearUnprintableChars(logToInclude, false)

	// origLogLen := len(logToInclude)
	// const maxLogLen = 256
	// if origLogLen > maxLogLen {
	// 	logToInclude = fmt.Sprintf("[%v chars trimmed]", origLogLen-maxLogLen) + logToInclude[len(logToInclude)-maxLogLen:]
	// }

	return fmt.Sprintf("(%v: '%v')", logLabel, logToInclude)
}
