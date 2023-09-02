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

package nettap

import (
	"crypto/rand"
	"fmt"
	"net"

	"github.com/pkg/errors"
)

type TapNet struct {
	HostIP  net.IP
	GuestIP net.IP

	HostCIDR  string
	GuestCIDR string
}

func GenerateNet() (TapNet, error) {
	// This is a Linsk internal network IPv6 prefix.
	hostIP := []byte(net.ParseIP("fe8f:5980:3253:7df4:0f4b:6db1::"))
	_, err := rand.Read(hostIP[len(hostIP)-4:])
	if err != nil {
		return TapNet{}, errors.Wrap(err, "random read")
	}

	// Put the last bit to zero.
	hostIP[len(hostIP)-1] &= 0xfe

	guestIP := make([]byte, len(hostIP))
	copy(guestIP, hostIP)

	// Put the last bit to one.
	guestIP[len(hostIP)-1] |= 0x1

	return TapNet{
		HostIP:  hostIP,
		GuestIP: guestIP,

		HostCIDR:  fmt.Sprintf("%v/127", net.IP(hostIP)),
		GuestCIDR: fmt.Sprintf("%v/127", net.IP(guestIP)),
	}, nil
}
