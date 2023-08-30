package constants

import "runtime"

func GetUnixWorkArch() string {
	arch := "x86_64"
	if runtime.GOOS == "arm64" {
		arch = "arm64"
	}

	// CPU architectures other than amd64 and arm64 are not yet natively supported.
	// Running on a non-officially-supported arch will result in use of x86_64 VM.
	return arch
}
