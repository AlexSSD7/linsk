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

func (vi *VM) ConfigureInterfaceStaticNet(ctx context.Context, iface string, cidr string) error {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return errors.Wrap(err, "invalid cidr")
	}

	if !utils.IsIPv6IP(ip) {
		return fmt.Errorf("ipv6 addresses accepted only (have '%v')", ip)
	}

	sc, err := vi.DialSSH()
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
