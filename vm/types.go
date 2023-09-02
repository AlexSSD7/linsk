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

package vm

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type PortForwardingRule struct {
	HostIP   net.IP
	HostPort uint16
	VMPort   uint16
}

func ParsePortForwardingRuleString(s string) (PortForwardingRule, error) {
	split := strings.Split(s, ":")
	switch len(split) {
	case 2:
		// Format: <HOST PORT>:<VM PORT>
		hostPort, err := strconv.ParseUint(split[0], 10, 16)
		if err != nil {
			return PortForwardingRule{}, errors.Wrap(err, "parse host port")
		}

		vmPort, err := strconv.ParseUint(split[1], 10, 16)
		if err != nil {
			return PortForwardingRule{}, errors.Wrap(err, "parse vm port")
		}

		return PortForwardingRule{
			HostPort: uint16(hostPort),
			VMPort:   uint16(vmPort),
		}, nil
	case 3:
		// Format: <HOST IP>:<HOST PORT>:<VM PORT>
		hostIP := net.ParseIP(split[0])
		if hostIP == nil {
			return PortForwardingRule{}, fmt.Errorf("bad host ip")
		}

		hostPort, err := strconv.ParseUint(split[1], 10, 16)
		if err != nil {
			return PortForwardingRule{}, errors.Wrap(err, "parse host port")
		}

		vmPort, err := strconv.ParseUint(split[2], 10, 16)
		if err != nil {
			return PortForwardingRule{}, errors.Wrap(err, "parse vm port")
		}

		return PortForwardingRule{
			HostIP:   hostIP,
			HostPort: uint16(hostPort),
			VMPort:   uint16(vmPort),
		}, nil
	default:
		return PortForwardingRule{}, fmt.Errorf("bad split by ':' length: want 2 or 3, have %v", len(split))
	}
}
