package osspecifics

import "runtime"

// For some reason, `runtime` package does not provide this while
// "goconst" linter complains about us not using constants in
// expressions like `runtime.GOOS == "windows"`. And it is
// not wrong, accidentally misspelling these OS IDs is a
// matter of time.

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}
