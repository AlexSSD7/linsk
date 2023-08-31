package utils

import "net"

func IsIPv6IP(ip net.IP) bool {
	return ip.To4() == nil && ip.To16() != nil
}
