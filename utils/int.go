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
