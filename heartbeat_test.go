package main

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock permite avançar o tempo manualmente nos testes.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestHeartbeat(clock *fakeClock, reportErr *error) (*heartbeat, *int, *atomic.Bool) {
	calls := 0
	paused := &atomic.Bool{}
	hb := newHeartbeat(func() error {
		calls++
		return *reportErr
	}, paused)
	hb.now = clock.now
	return hb, &calls, paused
}

func TestHeartbeat_NaoEnviaAntesDoIntervalo(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	var reportErr error
	hb, calls, _ := newTestHeartbeat(clock, &reportErr)
	hb.scheduleAfter(true) // reporte de startup bem-sucedido

	clock.advance(29 * time.Minute)
	hb.tick()

	if *calls != 0 {
		t.Fatalf("esperava 0 envios antes de 30min, obteve %d", *calls)
	}
}

func TestHeartbeat_EnviaQuandoVenceEReagenda(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	var reportErr error
	hb, calls, _ := newTestHeartbeat(clock, &reportErr)
	hb.scheduleAfter(true)

	clock.advance(30 * time.Minute)
	hb.tick()
	if *calls != 1 {
		t.Fatalf("esperava 1 envio em 30min, obteve %d", *calls)
	}

	// Ticks seguintes dentro do novo intervalo não reenviam
	clock.advance(1 * time.Minute)
	hb.tick()
	if *calls != 1 {
		t.Fatalf("esperava continuar em 1 envio, obteve %d", *calls)
	}
}

func TestHeartbeat_FalhaTentaDeNovoEm5Minutos(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	reportErr := errors.New("rede fora")
	hb, calls, _ := newTestHeartbeat(clock, &reportErr)
	hb.scheduleAfter(true)

	clock.advance(30 * time.Minute)
	hb.tick() // falha
	clock.advance(4 * time.Minute)
	hb.tick() // ainda dentro do retry
	if *calls != 1 {
		t.Fatalf("esperava 1 tentativa antes do retry, obteve %d", *calls)
	}

	reportErr = nil
	clock.advance(1 * time.Minute)
	hb.tick() // 5min depois da falha
	if *calls != 2 {
		t.Fatalf("esperava nova tentativa após 5min, obteve %d", *calls)
	}
}

func TestHeartbeat_StartupComFalhaTentaEm5Minutos(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	var reportErr error
	hb, calls, _ := newTestHeartbeat(clock, &reportErr)
	hb.scheduleAfter(false) // reporte de startup falhou

	clock.advance(5 * time.Minute)
	hb.tick()

	if *calls != 1 {
		t.Fatalf("esperava envio 5min após startup com falha, obteve %d", *calls)
	}
}

func TestHeartbeat_PausadoDuranteDesligamento(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	var reportErr error
	hb, calls, paused := newTestHeartbeat(clock, &reportErr)
	hb.scheduleAfter(true)
	paused.Store(true)

	clock.advance(2 * time.Hour)
	hb.tick()

	if *calls != 0 {
		t.Fatalf("heartbeat não pode reportar ON durante o desligamento, obteve %d envios", *calls)
	}
}

func TestHeartbeat_VoltaDeHibernacaoEnviaNoProximoTick(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	var reportErr error
	hb, calls, _ := newTestHeartbeat(clock, &reportErr)
	hb.scheduleAfter(true)

	// Máquina hiberna por 3 dias; ao acordar, o relógio de parede salta
	clock.advance(72 * time.Hour)
	hb.tick()

	if *calls != 1 {
		t.Fatalf("esperava 1 envio logo após acordar, obteve %d", *calls)
	}
}

func TestHeartbeat_IntervaloDeclaradoEh30Minutos(t *testing.T) {
	// O servidor usa 2x o valor declarado como limite para considerar OFF.
	// Mudar o intervalo aqui muda o limite do servidor junto (60 min hoje).
	if got := int(heartbeatInterval.Seconds()); got != 1800 {
		t.Fatalf("heartbeatSec = %d, esperava 1800", got)
	}
}
