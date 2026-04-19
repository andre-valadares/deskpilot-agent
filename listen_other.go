//go:build !windows

package main

import "net"

func listenWoL() (net.PacketConn, error) {
	return net.ListenPacket("udp4", ":9")
}
