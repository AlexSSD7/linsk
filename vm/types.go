package vm

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type USBDevicePassthroughConfig struct {
	HostBus  uint8
	HostPort uint8
}

type PortForwardingConfig struct {
	HostIP   net.IP
	HostPort uint16
	VMPort   uint16
}

func ParsePortForwardString(s string) (PortForwardingConfig, error) {
	split := strings.Split(s, ":")
	switch len(split) {
	case 2:
		// <HOST PORT>:<VM PORT>
		hostPort, err := strconv.ParseUint(split[0], 10, 16)
		if err != nil {
			return PortForwardingConfig{}, errors.Wrap(err, "parse host port")
		}

		vmPort, err := strconv.ParseUint(split[1], 10, 16)
		if err != nil {
			return PortForwardingConfig{}, errors.Wrap(err, "parse vm port")
		}

		return PortForwardingConfig{
			HostPort: uint16(hostPort),
			VMPort:   uint16(vmPort),
		}, nil
	case 3:
		// <HOST IP>:<HOST PORT>:<VM PORT>
		hostIP := net.ParseIP(split[0])
		if hostIP == nil {
			return PortForwardingConfig{}, fmt.Errorf("bad host ip")
		}

		hostPort, err := strconv.ParseUint(split[1], 10, 16)
		if err != nil {
			return PortForwardingConfig{}, errors.Wrap(err, "parse host port")
		}

		vmPort, err := strconv.ParseUint(split[2], 10, 16)
		if err != nil {
			return PortForwardingConfig{}, errors.Wrap(err, "parse vm port")
		}

		return PortForwardingConfig{
			HostIP:   hostIP,
			HostPort: uint16(hostPort),
			VMPort:   uint16(vmPort),
		}, nil
	default:
		return PortForwardingConfig{}, fmt.Errorf("bad split by ':' length: want 2 or 3, have %v", len(split))
	}
}
