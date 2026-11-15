package main

import (
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
)

// recorder registra a ordem das chamadas de reporte e shutdown.
type recorder struct {
	events    []string
	reportErr error
	shutErr   error
}

func (r *recorder) report(state string) error {
	r.events = append(r.events, "report:"+state)
	return r.reportErr
}

func (r *recorder) shutdown() error {
	r.events = append(r.events, "shutdown")
	return r.shutErr
}

func TestExecuteTurnOff_ReportaOFFAntesDoShutdown(t *testing.T) {
	r := &recorder{}
	var shuttingDown atomic.Bool

	executeTurnOff(r.report, r.shutdown, &shuttingDown)

	want := []string{"report:OFF", "shutdown"}
	if !reflect.DeepEqual(r.events, want) {
		t.Fatalf("ordem = %v, esperava %v", r.events, want)
	}
	if !shuttingDown.Load() {
		t.Fatal("shuttingDown deveria continuar true após shutdown iniciado")
	}
}

func TestExecuteTurnOff_DesligaMesmoSeReporteFalhar(t *testing.T) {
	r := &recorder{reportErr: errors.New("rede fora")}
	var shuttingDown atomic.Bool

	executeTurnOff(r.report, r.shutdown, &shuttingDown)

	want := []string{"report:OFF", "shutdown"}
	if !reflect.DeepEqual(r.events, want) {
		t.Fatalf("ordem = %v, esperava %v", r.events, want)
	}
}

func TestExecuteTurnOff_ShutdownFalhaRevertePraON(t *testing.T) {
	r := &recorder{shutErr: errors.New("acesso negado")}
	var shuttingDown atomic.Bool

	executeTurnOff(r.report, r.shutdown, &shuttingDown)

	want := []string{"report:OFF", "shutdown", "report:ON"}
	if !reflect.DeepEqual(r.events, want) {
		t.Fatalf("ordem = %v, esperava %v", r.events, want)
	}
	if shuttingDown.Load() {
		t.Fatal("shuttingDown deveria voltar a false para o heartbeat retomar")
	}
}
