//go:build windows

package main

import (
	"context"
	"log"
	"net"
	"syscall"

	"golang.org/x/sys/windows"
)

func listenWoL() (net.PacketConn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var setsockoptErr error
			err := c.Control(func(fd uintptr) {
				log.Printf("listenWoL: Control callback fd=%d", fd)
				setsockoptErr = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_BROADCAST, 1)
				if setsockoptErr != nil {
					log.Printf("listenWoL: SO_BROADCAST falhou: %v", setsockoptErr)
				} else {
					log.Println("listenWoL: SO_BROADCAST OK")
				}
			})
			if err != nil {
				return err
			}
			return setsockoptErr
		},
	}
	conn, err := lc.ListenPacket(context.Background(), "udp4", ":9")
	if err == nil {
		log.Println("listenWoL: socket UDP:9 aberto com sucesso")
	}
	return conn, err
}
