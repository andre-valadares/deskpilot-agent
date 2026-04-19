//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"syscall"

	"golang.org/x/sys/windows"
)

func runWoLListener(macs []string, onWoL func()) {
	conn, err := net.ListenPacket("ip4:17", "0.0.0.0")
	if err != nil {
		log.Printf("raw socket indisponível (%v) — usando UDP com SO_BROADCAST", err)
		runUDPWoLListener(macs, onWoL)
		return
	}
	defer conn.Close()
	log.Println("listener: raw socket ip4:17 ativo")

	buf := make([]byte, 65536)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("erro lendo raw socket: %v", err)
			continue
		}
		pkt := buf[:n]

		// Cabeçalho IP — versão e IHL
		if len(pkt) < 20 {
			continue
		}
		if pkt[0]>>4 != 4 {
			continue
		}
		ihl := int(pkt[0]&0x0f) * 4
		if len(pkt) < ihl+8 {
			continue
		}

		// Porta UDP de destino
		dstPort := binary.BigEndian.Uint16(pkt[ihl+2 : ihl+4])
		if dstPort != 9 {
			continue
		}

		payload := pkt[ihl+8:]
		srcIP := fmt.Sprintf("%d.%d.%d.%d", pkt[12], pkt[13], pkt[14], pkt[15])
		dstIP := fmt.Sprintf("%d.%d.%d.%d", pkt[16], pkt[17], pkt[18], pkt[19])

		if len(payload) < 102 {
			log.Printf("pacote UDP:9 de %s→%s ignorado — payload %d < 102 bytes", srcIP, dstIP, len(payload))
			continue
		}

		mac := extractMACFromWoL(payload)
		if mac == "" {
			log.Printf("pacote UDP:9 de %s→%s ignorado — não é magic packet válido", srcIP, dstIP)
			continue
		}

		if !containsMAC(macs, mac) {
			log.Printf("WoL de %s→%s ignorado — MAC alvo %s não pertence a este host", srcIP, dstIP, mac)
			continue
		}

		log.Printf("WoL recebido de %s (via %s) para %s", srcIP, addr, mac)
		onWoL()
	}
}

func runUDPWoLListener(macs []string, onWoL func()) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				if err := windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_BROADCAST, 1); err != nil {
					log.Printf("SO_BROADCAST: %v", err)
				}
			})
		},
	}
	conn, err := lc.ListenPacket(context.Background(), "udp4", ":9")
	if err != nil {
		log.Fatalf("erro ao escutar UDP:9: %v", err)
	}
	defer conn.Close()
	log.Println("listener: UDP socket :9 ativo (fallback)")

	buf := make([]byte, 102)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("erro lendo UDP: %v", err)
			continue
		}
		if n < 102 {
			log.Printf("pacote UDP de %s ignorado — tamanho %d < 102 bytes", addr, n)
			continue
		}
		mac := extractMACFromWoL(buf[:n])
		if mac == "" {
			log.Printf("pacote UDP de %s ignorado — não é magic packet válido", addr)
			continue
		}
		if !containsMAC(macs, mac) {
			log.Printf("WoL de %s ignorado — MAC alvo %s não pertence a este host", addr, mac)
			continue
		}
		log.Printf("WoL recebido de %s para %s", addr, mac)
		onWoL()
	}
}
