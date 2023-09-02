package share

import (
	"net"

	"github.com/AlexSSD7/linsk/osspecifics"
)

func IsSMBExtModeDefault() bool {
	return osspecifics.IsWindows()
}

var defaultListenIP = net.ParseIP("127.0.0.1")

func GetDefaultListenIPStr() string {
	return defaultListenIP.String()
}
