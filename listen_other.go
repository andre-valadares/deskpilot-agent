//go:build !windows

package main

import (
	"log"
	"net"
)

func runWoLListener(macs []string, onWoL func()) {
	conn, err := net.ListenPacket("udp4", ":9")
	if err != nil {
		log.Fatalf("erro ao escutar UDP:9: %v — execute como root/administrador", err)
	}
	defer conn.Close()
	log.Println("listener: UDP socket :9 ativo")

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
