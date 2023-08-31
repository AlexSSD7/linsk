package share

import (
	"fmt"
	"net"

	"log/slog"
)

var defaultListenIP = net.ParseIP("127.0.0.1")

func GetDefaultListenIPStr() string {
	return defaultListenIP.String()
}

type UserConfiguration struct {
	listenIP net.IP
	ftpExtIP net.IP

	smbExtMode bool
}

type RawUserConfiguration struct {
	ListenIP string

	// Backend-specific
	FTPExtIP   string
	SMBExtMode bool
}

func (rc RawUserConfiguration) Process(backend string, warnLogger *slog.Logger) (*UserConfiguration, error) {
	listenIP := net.ParseIP(rc.ListenIP)
	if listenIP == nil {
		return nil, fmt.Errorf("invalid listen ip '%v'", rc.ListenIP)
	}

	ftpExtIP := net.ParseIP(rc.FTPExtIP)
	if ftpExtIP == nil {
		return nil, fmt.Errorf("invalid ftp ext ip '%v'", rc.FTPExtIP)
	}

	if backend == "ftp" {
		if !listenIP.Equal(defaultListenIP) && ftpExtIP.Equal(defaultListenIP) {
			slog.Warn("No external FTP IP address via --ftp-extip was configured. This is a requirement in almost all scenarios if you want to connect remotely.")
		}
	} else {
		if !ftpExtIP.Equal(defaultListenIP) {
			slog.Warn("FTP external IP address specification is ineffective with non-FTP backends", "selected", backend)
		}
	}

	return &UserConfiguration{
		listenIP:   listenIP,
		ftpExtIP:   ftpExtIP,
		smbExtMode: rc.SMBExtMode,
	}, nil
}
