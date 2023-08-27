package utils

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

func WrapErrWithLog(err error, msg, log string) error {
	return errors.Wrapf(err, "%v %v", msg, GetLogErrMsg(log))
}

func GetLogErrMsg(s string) string {
	logToInclude := strings.ReplaceAll(s, "\n", "\\n")
	logToInclude = strings.TrimSuffix(logToInclude, "\\n")
	logToInclude = ClearUnprintableChars(logToInclude, false)

	origLogLen := len(logToInclude)
	const maxLogLen = 256
	if origLogLen > maxLogLen {
		logToInclude = fmt.Sprintf("[%v chars trimmed]", origLogLen) + logToInclude[len(logToInclude)-maxLogLen:]
	}

	return fmt.Sprintf("(log: '%v')", logToInclude)
}
