// Linsk - A utility to access Linux-native file systems on non-Linux operating systems.
// Copyright (c) 2023 The Linsk Authors.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package share

import (
	"fmt"
	"net"

	"log/slog"
)

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
			warnLogger.Warn("No external FTP IP address via --ftp-extip was configured. This is a requirement in almost all scenarios if you want to connect remotely.")
		}
	} else {
		if !ftpExtIP.Equal(defaultListenIP) {
			warnLogger.Warn("FTP external IP address specification is ineffective with non-FTP backends", "selected", backend)
		}
	}

	if rc.SMBExtMode && backend != "smb" && !IsSMBExtModeDefault() {
		warnLogger.Warn("SMB external mode specification is ineffective with non-SMB backends")
	}

	return &UserConfiguration{
		listenIP:   listenIP,
		ftpExtIP:   ftpExtIP,
		smbExtMode: rc.SMBExtMode,
	}, nil
}
