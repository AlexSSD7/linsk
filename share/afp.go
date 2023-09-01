package share

import (
	"context"
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

func (b *AFPBackend) Apply(ctx context.Context, sharePWD string, vc *VMShareContext) (string, error) {
	err := vc.FileManager.StartAFP(sharePWD)
	if err != nil {
		return "", errors.Wrap(err, "start afp server")
	}

	return "afp://" + net.JoinHostPort(b.listenIP.String(), fmt.Sprint(b.sharePort)) + "/linsk", nil
}
