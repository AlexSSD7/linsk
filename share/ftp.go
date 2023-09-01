package share

import (
	"context"
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
			Ports:     ports,
			EnableTap: false,
		}, nil
}

func (b *FTPBackend) Apply(ctx context.Context, sharePWD string, vc *VMShareContext) (string, error) {
	if vc.NetTapCtx != nil {
		return "", fmt.Errorf("net taps are unsupported in ftp")
	}

	err := vc.FileManager.StartFTP(sharePWD, b.sharePort+1, b.passivePortCount, b.extIP)
	if err != nil {
		return "", errors.Wrap(err, "start ftp server")
	}

	return "ftp://" + b.extIP.String() + ":" + fmt.Sprint(b.sharePort), nil
}
