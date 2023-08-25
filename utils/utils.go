package utils

import (
	"regexp"
	"strings"
	"unicode"
)

func ClearUnprintableChars(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
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
