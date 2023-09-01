package share

import (
	"net"
	"runtime"
)

func IsSMBExtModeDefault() bool {
	return runtime.GOOS == "windows"
}

var defaultListenIP = net.ParseIP("127.0.0.1")

func GetDefaultListenIPStr() string {
	return defaultListenIP.String()
}
