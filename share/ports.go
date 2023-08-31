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
	// We use 10 as port range
	for i := port; i < 65535; i += subsequent {
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
		return false, fmt.Errorf("subsequent ports exceed allowed port range")
	}

	if subsequent == 0 {
		ln, err := net.Listen("tcp", ":"+fmt.Sprint(port))
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok {
				if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
					if sysErr.Err == syscall.EADDRINUSE {
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
