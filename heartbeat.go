package main

import (
	"log"
	"sync/atomic"
	"time"
)

const (
	// Intervalo entre sinais de vida. Do lado do servidor, sem sinal em
	// 2x este intervalo (60 min) o device passa a ser considerado OFF.
	heartbeatInterval = 30 * time.Minute
	// Espera antes de tentar de novo após falha (rede fora, API indisponível).
	heartbeatRetry = 5 * time.Minute
	// Frequência da checagem local. Não gera tráfego: só envia quando vencido.
	// Checar pelo relógio de parede a cada minuto detecta a volta de
	// hibernação/Inicialização Rápida rapidamente — timers do Go no Windows
	// podem não contar o tempo em que a máquina dormiu.
	heartbeatCheckEvery = 1 * time.Minute
)

type heartbeat struct {
	interval time.Duration
	retry    time.Duration
	now      func() time.Time
	report   func() error
	paused   *atomic.Bool // true durante o desligamento — nunca reportar ON depois do OFF
	next     time.Time
}

func newHeartbeat(report func() error, paused *atomic.Bool) *heartbeat {
	return &heartbeat{
		interval: heartbeatInterval,
		retry:    heartbeatRetry,
		now:      time.Now,
		report:   report,
		paused:   paused,
	}
}

// wallNow descarta a leitura monotônica para que as comparações usem o
// relógio de parede, que avança durante suspensão/hibernação.
func (h *heartbeat) wallNow() time.Time {
	return h.now().Round(0)
}

// scheduleAfter agenda o próximo envio conforme o resultado do último reporte.
func (h *heartbeat) scheduleAfter(ok bool) {
	if ok {
		h.next = h.wallNow().Add(h.interval)
	} else {
		h.next = h.wallNow().Add(h.retry)
	}
}

// tick envia o sinal de vida se estiver vencido.
func (h *heartbeat) tick() {
	if h.paused.Load() {
		return
	}
	if h.wallNow().Before(h.next) {
		return
	}
	if err := h.report(); err != nil {
		log.Printf("heartbeat falhou: %v — nova tentativa em %s", err, h.retry)
		h.scheduleAfter(false)
		return
	}
	log.Println("heartbeat: OK")
	h.scheduleAfter(true)
}

func (h *heartbeat) run() {
	t := time.NewTicker(heartbeatCheckEvery)
	defer t.Stop()
	for range t.C {
		h.tick()
	}
}
