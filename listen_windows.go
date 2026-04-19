//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"log"
	"net"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const sioRcvAll = 0x98000001 // SIO_RCVALL — promiscuous mode, recebe directed broadcast

func runWoLListener(macs []string, onWoL func()) {
	var started int
	for _, ip := range localIPv4s() {
		conn, err := net.ListenPacket("ip4:0", ip)
		if err != nil {
			log.Printf("ip4:0 em %s falhou: %v", ip, err)
			continue
		}

		if ok := enableRcvAll(conn); !ok {
			conn.Close()
			continue
		}

		log.Printf("listener: SIO_RCVALL ativo em %s (inclui directed broadcast)", ip)
		started++
		go runRawLoop(conn, macs, onWoL)
	}

	if started > 0 {
		select {} // bloqueia; goroutines acima fazem o trabalho
	}

	log.Println("SIO_RCVALL indisponível em todas as interfaces — usando ip4:17 fallback")
	runIP17Listener(macs, onWoL)
}

func enableRcvAll(conn net.PacketConn) bool {
	ipConn, ok := conn.(*net.IPConn)
	if !ok {
		return false
	}
	raw, err := ipConn.SyscallConn()
	if err != nil {
		return false
	}
	var success bool
	_ = raw.Control(func(fd uintptr) {
		enable := uint32(1)
		var returned uint32
		err := windows.WSAIoctl(
			windows.Handle(fd), sioRcvAll,
			(*byte)(unsafe.Pointer(&enable)), 4,
			nil, 0, &returned, nil, 0,
		)
		if err != nil {
			log.Printf("WSAIoctl SIO_RCVALL: %v", err)
		} else {
			success = true
		}
	})
	return success
}

func runRawLoop(conn net.PacketConn, macs []string, onWoL func()) {
	defer conn.Close()
	// SIO_RCVALL entrega pacotes IP completos (com cabeçalho IP)
	buf := make([]byte, 65536)
	var pktCount uint64
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("erro lendo SIO_RCVALL: %v", err)
			return
		}
		pktCount++
		pkt := buf[:n]
		if len(pkt) < 20 {
			continue
		}
		proto := pkt[9]
		if pktCount <= 5 {
			dump := pkt
			if len(dump) > 32 {
				dump = dump[:32]
			}
			log.Printf("SIO_RCVALL pkt#%d de %s, %d bytes, proto=%d, hex=%s",
				pktCount, addr, n, proto, hex.EncodeToString(dump))
		}
		if pkt[0]>>4 != 4 || proto != 17 { // IPv4 + UDP
			continue
		}
		ihl := int(pkt[0]&0x0f) * 4
		if len(pkt) < ihl+8 {
			continue
		}
		dstPort := binary.BigEndian.Uint16(pkt[ihl+2 : ihl+4])
		log.Printf("SIO_RCVALL UDP de %s, dstPort=%d, payload=%d bytes", addr, dstPort, n-ihl-8)
		if dstPort != 9 {
			continue
		}
		payload := pkt[ihl+8:]
		if len(payload) < 102 {
			log.Printf("pacote UDP:9 de %s ignorado — payload %d < 102 bytes", addr, len(payload))
			continue
		}
		mac := extractMACFromWoL(payload)
		if mac == "" {
			log.Printf("pacote UDP:9 de %s ignorado — não é magic packet válido", addr)
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

// runIP17Listener — fallback: raw socket sem SIO_RCVALL
// Go's ip4:17 no Windows entrega UDP sem cabeçalho IP: [0:2] src port | [2:4] dst port | [8:] payload
func runIP17Listener(macs []string, onWoL func()) {
	conn, err := net.ListenPacket("ip4:17", "0.0.0.0")
	if err != nil {
		log.Printf("ip4:17 falhou: %v — usando UDP fallback final", err)
		runUDPWoLListener(macs, onWoL)
		return
	}
	defer conn.Close()
	log.Println("listener: ip4:17 ativo (sem IP header)")

	buf := make([]byte, 65536)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("erro lendo ip4:17: %v", err)
			continue
		}
		if n < 8 {
			continue
		}
		if binary.BigEndian.Uint16(buf[2:4]) != 9 {
			continue
		}
		payload := buf[8:n]
		if len(payload) < 102 {
			continue
		}
		mac := extractMACFromWoL(payload)
		if mac == "" || !containsMAC(macs, mac) {
			continue
		}
		log.Printf("WoL recebido de %s para %s", addr, mac)
		onWoL()
	}
}

func runUDPWoLListener(macs []string, onWoL func()) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_BROADCAST, 1)
			})
		},
	}
	conn, err := lc.ListenPacket(context.Background(), "udp4", ":9")
	if err != nil {
		log.Fatalf("erro ao escutar UDP:9: %v", err)
	}
	defer conn.Close()
	log.Println("listener: UDP :9 ativo (último fallback)")

	buf := make([]byte, 102)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("erro lendo UDP: %v", err)
			continue
		}
		if n < 102 {
			continue
		}
		mac := extractMACFromWoL(buf[:n])
		if mac == "" || !containsMAC(macs, mac) {
			continue
		}
		log.Printf("WoL recebido de %s para %s", addr, mac)
		onWoL()
	}
}

func localIPv4s() []string {
	ifaces, _ := net.Interfaces()
	var ips []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 ||
			iface.Flags&net.FlagUp == 0 ||
			iface.Flags&net.FlagBroadcast == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil || ip4[0] == 169 { // descarta link-local
				continue
			}
			ips = append(ips, ip4.String())
		}
	}
	return ips
}
