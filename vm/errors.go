package vm

import (
	"fmt"
	"strings"

	"github.com/AlexSSD7/linsk/utils"
	"github.com/pkg/errors"
)

var (
	ErrSSHUnavailable = errors.New("ssh unavailable")
)

func wrapErrWithLog(err error, msg, log string) error {
	return errors.Wrapf(err, "%v %v", msg, getLogErrMsg(log))
}

func getLogErrMsg(s string) string {
	logToInclude := strings.ReplaceAll(s, "\n", "\\n")
	logToInclude = strings.TrimSuffix(logToInclude, "\\n")
	logToInclude = utils.ClearUnprintableChars(logToInclude, false)

	origLogLen := len(logToInclude)
	const maxLogLen = 256
	if origLogLen > maxLogLen {
		logToInclude = fmt.Sprintf("[%v chars trimmed]", origLogLen) + logToInclude[len(logToInclude)-maxLogLen:]
	}

	return fmt.Sprintf("(log: '%v')", logToInclude)
}
