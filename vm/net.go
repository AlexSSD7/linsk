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
	"context"
	"fmt"
	"net"

	"github.com/AlexSSD7/linsk/sshutil"
	"github.com/AlexSSD7/linsk/utils"
	"github.com/alessio/shellescape"
	"github.com/pkg/errors"
)

func (vm *VM) ConfigureInterfaceStaticNet(ctx context.Context, iface string, cidr string) error {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return errors.Wrap(err, "invalid cidr")
	}

	if !utils.IsIPv6IP(ip) {
		return fmt.Errorf("ipv6 addresses accepted only (have '%v')", ip)
	}

	sc, err := vm.DialSSH()
	if err != nil {
		return errors.Wrap(err, "dial ssh")
	}

	defer func() { _ = sc.Close() }()

	_, err = sshutil.RunSSHCmd(ctx, sc, "ifconfig "+shellescape.Quote(iface)+" up && ip addr add "+shellescape.Quote(cidr)+" dev "+shellescape.Quote(iface))
	if err != nil {
		return errors.Wrap(err, "run net conf cmds")
	}

	return nil
}
