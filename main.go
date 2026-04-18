package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func main() {
	token := flag.String("token", "", "agent token")
	apiURL := flag.String("api", "", "DeskPilot API URL (e.g. https://wol.deskpilot.xyz)")
	install := flag.Bool("install", false, "save config and register as system service")
	debug := flag.Bool("debug", false, "enable file logging (persisted in config)")
	flag.Parse()

	if *install {
		if *token == "" || *apiURL == "" {
			log.Fatal("--token e --api são obrigatórios para --install")
		}
		cfg := &Config{Token: *token, ApiURL: strings.TrimRight(*apiURL, "/"), Debug: *debug}
		if err := SaveConfig(cfg); err != nil {
			log.Fatalf("erro ao salvar config: %v", err)
		}
		fmt.Println("Config salva em", configPath())
		fmt.Println("Execute o script de instalação de serviço para o seu OS.")
		return
	}

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config não encontrada — rode com --token e --api --install primeiro: %v", err)
	}

	if cfg.Debug {
		setupFileLogging()
	}

	log.Printf("config carregada: api=%s debug=%v", cfg.ApiURL, cfg.Debug)

	macs, err := ownMACAddresses()
	if err != nil {
		log.Fatalf("erro ao obter MACs locais: %v", err)
	}
	log.Printf("MACs monitorados: %v", macs)

	if err := reportState(cfg, "ON"); err != nil {
		log.Printf("aviso: reportState ON falhou: %v", err)
	} else {
		log.Println("reportState ON: OK")
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigs
		log.Println("sinal recebido — reportando OFF")
		_ = reportState(cfg, "OFF")
		os.Exit(0)
	}()

	conn, err := net.ListenPacket("udp4", ":9")
	if err != nil {
		log.Fatalf("erro ao escutar UDP:9: %v — execute como root/administrador", err)
	}
	defer conn.Close()
	log.Println("aguardando pacotes WoL na porta 9...")

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
		go handleWoL(cfg)
	}
}

func handleWoL(cfg *Config) {
	cmd, err := readCommand(cfg)
	if err != nil {
		log.Printf("erro ao ler comando: %v", err)
		return
	}
	log.Printf("pendingCommand: %q", cmd)

	switch cmd {
	case "":
		log.Println("nenhum comando pendente — WoL ignorado")
	case "TurnOff":
		log.Println("executando shutdown...")
		if err := shutdown(); err != nil {
			log.Printf("erro no shutdown: %v", err)
			return
		}
		log.Println("shutdown iniciado com sucesso")
		if err := reportState(cfg, "OFF"); err != nil {
			log.Printf("erro ao reportar OFF: %v", err)
		} else {
			log.Println("reportState OFF: OK")
		}
	default:
		log.Printf("comando desconhecido %q — ignorado", cmd)
	}
}

func shutdown() error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command(`C:\Windows\System32\shutdown.exe`, "/s", "/t", "0")
	default:
		c = exec.Command("shutdown", "-h", "now")
	}
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w; output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func logPath() string {
	programData := os.Getenv("ProgramData")
	if programData != "" {
		return filepath.Join(programData, "DeskPilot", "agent.log")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".deskpilot", "agent.log")
}

func setupFileLogging() {
	path := logPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("log iniciado em %s", path)
}

func ownMACAddresses() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var macs []string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		macs = append(macs, normalizeMACBytes(iface.HardwareAddr))
	}
	return macs, nil
}

func normalizeMACBytes(hw net.HardwareAddr) string {
	return strings.ToUpper(hw.String())
}

// WoL magic packet: 6 bytes 0xFF + 16 repetições do MAC de 6 bytes = 102 bytes
func extractMACFromWoL(pkt []byte) string {
	if len(pkt) < 102 {
		return ""
	}
	for i := 0; i < 6; i++ {
		if pkt[i] != 0xFF {
			return ""
		}
	}
	macBytes := pkt[6:12]
	for rep := 1; rep < 16; rep++ {
		if string(pkt[6+rep*6:12+rep*6]) != string(macBytes) {
			return ""
		}
	}
	return strings.ToUpper(hex.EncodeToString(macBytes[:1]) + ":" +
		hex.EncodeToString(macBytes[1:2]) + ":" +
		hex.EncodeToString(macBytes[2:3]) + ":" +
		hex.EncodeToString(macBytes[3:4]) + ":" +
		hex.EncodeToString(macBytes[4:5]) + ":" +
		hex.EncodeToString(macBytes[5:6]))
}

func containsMAC(macs []string, target string) bool {
	for _, m := range macs {
		if m == target {
			return true
		}
	}
	return false
}
