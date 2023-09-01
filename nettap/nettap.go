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
