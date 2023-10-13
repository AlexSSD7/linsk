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
	"strings"

	"github.com/AlexSSD7/linsk/osspecifics"
	"github.com/AlexSSD7/linsk/vm"
	"github.com/pkg/errors"
)

const smbPort = 445

type SMBBackend struct {
	listenIP  net.IP
	sharePort *uint16
}

func NewSMBBackend(uc *UserConfiguration) (Backend, *VMShareOptions, error) {
	var ports []vm.PortForwardingRule
	var sharePortPtr *uint16
	if !uc.smbExtMode {
		sharePort, err := getNetworkSharePort(0)
		if err != nil {
			return nil, nil, errors.Wrap(err, "get network share port")
		}

		sharePortPtr = &sharePort

		ports = append(ports, vm.PortForwardingRule{
			HostIP:   uc.listenIP,
			HostPort: sharePort,
			VMPort:   smbPort,
		})
	}

	return &SMBBackend{
			listenIP:  uc.listenIP,
			sharePort: sharePortPtr,
		}, &VMShareOptions{
			Ports:     ports,
			EnableTap: uc.smbExtMode,
		}, nil
}

func (b *SMBBackend) Apply(sharePWD string, vc *VMShareContext) (string, error) {
	if b.sharePort != nil && vc.NetTapCtx != nil {
		return "", fmt.Errorf("conflict: configured to use a forwarded port but a net tap configuration was detected")
	}

	if b.sharePort == nil && vc.NetTapCtx == nil {
		return "", fmt.Errorf("no net tap configuration found")
	}

	err := vc.FileManager.StartSMB(sharePWD)
	if err != nil {
		return "", errors.Wrap(err, "start smb server")
	}

	var shareURL string
	switch {
	case b.sharePort != nil:
		shareURL = "smb://" + net.JoinHostPort(b.listenIP.String(), fmt.Sprint(*b.sharePort)) + "/linsk"
	case vc.NetTapCtx != nil:
		if osspecifics.IsWindows() {
			shareURL = `\\` + strings.ReplaceAll(vc.NetTapCtx.Net.GuestIP.String(), ":", "-") + ".ipv6-literal.net" + `\linsk`
		} else {
			shareURL = "smb://" + net.JoinHostPort(vc.NetTapCtx.Net.GuestIP.String(), fmt.Sprint(smbPort)) + "/linsk"
		}
	default:
		return "", fmt.Errorf("no port forwarding and net tap configured")
	}

	return shareURL, nil
}
