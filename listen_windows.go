//go:build windows

package main

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/windows"
)

func listenWoL() (net.PacketConn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_BROADCAST, 1)
			})
		},
	}
	return lc.ListenPacket(context.Background(), "udp4", ":9")
}
