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

	"github.com/AlexSSD7/linsk/vm"
	"github.com/pkg/errors"
)

type FTPBackend struct {
	sharePort        uint16
	passivePortCount uint16
	extIP            net.IP
}

func NewFTPBackend(uc *UserConfiguration) (Backend, *VMShareOptions, error) {
	// TODO: Make this changeable?
	passivePortCount := uint16(9)

	sharePort, err := getNetworkSharePort(9)
	if err != nil {
		return nil, nil, errors.Wrap(err, "get network share port")
	}

	ports := []vm.PortForwardingRule{{
		HostIP:   uc.listenIP,
		HostPort: sharePort,
		VMPort:   21,
	}}

	for i := uint16(0); i < passivePortCount; i++ {
		p := sharePort + 1 + i
		ports = append(ports, vm.PortForwardingRule{
			HostIP:   uc.listenIP,
			HostPort: p,
			VMPort:   p,
		})
	}

	return &FTPBackend{
			sharePort:        sharePort,
			passivePortCount: passivePortCount,
			extIP:            uc.ftpExtIP,
		}, &VMShareOptions{
			Ports: ports,
		}, nil
}

func (b *FTPBackend) Apply(sharePWD string, vc *VMShareContext) (string, error) {
	if vc.NetTapCtx != nil {
		return "", fmt.Errorf("net taps are unsupported in ftp")
	}

	err := vc.FileManager.StartFTP(sharePWD, b.sharePort+1, b.passivePortCount, b.extIP)
	if err != nil {
		return "", errors.Wrap(err, "start ftp server")
	}

	return "ftp://" + b.extIP.String() + ":" + fmt.Sprint(b.sharePort), nil
}
