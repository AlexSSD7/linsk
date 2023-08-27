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

var devNameRegexp = regexp.MustCompile("^[0-9a-z_-]+$")

func ValidateDevName(s string) bool {
	// Allow mapped devices.
	s = strings.TrimPrefix(s, "mapper/")

	return devNameRegexp.MatchString(s)
}

func Uint16ToBytesBE(v uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	return b
}
