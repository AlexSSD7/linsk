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

	return nil
}
