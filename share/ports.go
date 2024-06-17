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
	"os"
	"syscall"

	"github.com/pkg/errors"
)

func getNetworkSharePort(subsequent uint16) (uint16, error) {
	return getClosestAvailPortWithSubsequent(9000, subsequent)
}

func getClosestAvailPortWithSubsequent(port uint16, subsequent uint16) (uint16, error) {
	for i := port; i < 65535; i += 1 + subsequent {
		ok, err := checkPortAvailable(i, subsequent)
		if err != nil {
			return 0, errors.Wrapf(err, "check port available (%v)", i)
		}

		if ok {
			return i, nil
		}
	}

	return 0, fmt.Errorf("no available port (with %v subsequent ones) found", subsequent)
}

func checkPortAvailable(port uint16, subsequent uint16) (bool, error) {
	if port+subsequent < port {
		// We check for uint16 overflow here.
		return false, fmt.Errorf("subsequent ports exceed allowed port range")
	}

	if subsequent == 0 {
		ln, err := net.Listen("tcp", "127.0.0.1:"+fmt.Sprint(port))
		if err != nil {
			opErr := new(net.OpError)
			if errors.As(err, &opErr) {
				sysErr := new(os.SyscallError)
				if errors.As(opErr.Err, &sysErr) {
					if errors.Is(sysErr.Err, syscall.EADDRINUSE) {
						// The port is in use.
						return false, nil
					}
				}
			}

			return false, errors.Wrapf(err, "net listen (port %v)", port)
		}

		err = ln.Close()
		if err != nil {
			return false, errors.Wrap(err, "close ephemeral listener")
		}

		return true, nil
	}

	for i := uint16(0); i < subsequent; i++ {
		ok, err := checkPortAvailable(port+i, 0)
		if err != nil {
			return false, errors.Wrapf(err, "check subsequent port available (base: %v, seq: %v)", port, i)
		}

		if !ok {
			return false, nil
		}
	}

	return true, nil
}
