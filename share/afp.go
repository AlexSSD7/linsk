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

type AFPBackend struct {
	listenIP  net.IP
	sharePort uint16
}

func NewAFPBackend(uc *UserConfiguration) (Backend, *VMShareOptions, error) {
	sharePort, err := getNetworkSharePort(0)
	if err != nil {
		return nil, nil, errors.Wrap(err, "get network share port")
	}

	return &AFPBackend{
			listenIP:  uc.listenIP,
			sharePort: sharePort,
		}, &VMShareOptions{
			Ports: []vm.PortForwardingRule{{
				HostIP:   uc.listenIP,
				HostPort: sharePort,
				VMPort:   548,
			}},
		}, nil
}

func (b *AFPBackend) Apply(sharePWD string, vc *VMShareContext) (string, error) {
	err := vc.FileManager.StartAFP(sharePWD)
	if err != nil {
		return "", errors.Wrap(err, "start afp server")
	}

	return "afp://" + net.JoinHostPort(b.listenIP.String(), fmt.Sprint(b.sharePort)) + "/linsk", nil
}
