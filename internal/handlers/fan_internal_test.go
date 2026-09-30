package handlers

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/fan"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/sysstats"
)

type fakeRelay struct {
	onCalls, offCalls int
	setErr            error
}

func (f *fakeRelay) Set(on bool) error {
	if on {
		f.onCalls++
	} else {
		f.offCalls++
	}
	return f.setErr
}

type fakeFanStats struct {
	tempC float64
}

func (f *fakeFanStats) Snapshot() (sysstats.Snapshot, error) {
	return sysstats.Snapshot{CPUTempC: f.tempC}, nil
}

type fixedFanClock struct{ t time.Time }

func (f fixedFanClock) Now() time.Time { return f.t }

func newTestFanMachine(t *testing.T, mode string) (*fan.Machine, *fakeRelay) {
	t.Helper()
	relay := &fakeRelay{}
	m, err := fan.NewMachine(relay, &fakeFanStats{tempC: 50}, fixedFanClock{t: time.Now()}, mode)
	if err != nil {
		t.Fatalf("fan.NewMachine: %v", err)
	}
	return m, relay
}

func TestFanHandlersOnPersistsStateAndCallsRelay(t *testing.T) {
	machine, relay := newTestFanMachine(t, fan.ModeOff)
	cfg := config.Default()
	cfg.Fan.CaseFanState = fan.ModeOff
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, FanHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "fan.on", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if relay.onCalls != 1 {
		t.Fatalf("expected relay.Set(true) to be called once, got %d", relay.onCalls)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.Fan.CaseFanState != fan.ModeOn {
		t.Fatalf("expected the saved config to have fan.case_fan_state=on, got %q", saved.Fan.CaseFanState)
	}
}

func TestFanHandlersOffPersistsStateAndCallsRelay(t *testing.T) {
	machine, relay := newTestFanMachine(t, fan.ModeOn)
	cfg := config.Default()
	cfg.Fan.CaseFanState = fan.ModeOn
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, FanHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "fan.off", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if relay.offCalls != 1 {
		t.Fatalf("expected relay.Set(false) to be called once, got %d", relay.offCalls)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.Fan.CaseFanState != fan.ModeOff {
		t.Fatalf("expected the saved config to have fan.case_fan_state=off, got %q", saved.Fan.CaseFanState)
	}
}

func TestFanHandlersAutoPersistsStateAndEvaluatesTemperature(t *testing.T) {
	relay := &fakeRelay{}
	machine, err := fan.NewMachine(relay, &fakeFanStats{tempC: 70}, fixedFanClock{t: time.Now()}, fan.ModeOff)
	if err != nil {
		t.Fatalf("fan.NewMachine: %v", err)
	}
	cfg := config.Default()
	cfg.Fan.CaseFanState = fan.ModeOff
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, FanHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "fan.auto", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if relay.onCalls != 1 {
		t.Fatalf("expected auto mode to energize the relay above the threshold, got %d On() calls", relay.onCalls)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.Fan.CaseFanState != fan.ModeAuto {
		t.Fatalf("expected the saved config to have fan.case_fan_state=auto, got %q", saved.Fan.CaseFanState)
	}
}

func TestFanHandlersPropagatesPersistErrorWithoutTouchingRelay(t *testing.T) {
	machine, relay := newTestFanMachine(t, fan.ModeOff)
	cfg := config.Default()
	cfg.Fan.CaseFanState = fan.ModeOff

	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfgPath := filepath.Join(blocker, "config.yaml")

	path := startTestDaemon(t, FanHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "fan.on", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false when persisting fails")
	}
	if cfg.Fan.CaseFanState != fan.ModeOff {
		t.Fatalf("expected cfg to remain unchanged when persist fails, got %q", cfg.Fan.CaseFanState)
	}
	if relay.onCalls != 0 {
		t.Fatalf("expected the relay to never be touched when persisting fails, got %d On() calls", relay.onCalls)
	}
}

func TestFanHandlersCommitsPersistedStateEvenWhenRelayFails(t *testing.T) {
	relay := &fakeRelay{}
	m, err := fan.NewMachine(relay, &fakeFanStats{tempC: 50}, fixedFanClock{t: time.Now()}, fan.ModeOff)
	if err != nil {
		t.Fatalf("fan.NewMachine: %v", err)
	}
	relay.setErr = errors.New("gpio write failed")

	cfg := config.Default()
	cfg.Fan.CaseFanState = fan.ModeOff
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, FanHandlers(m, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "fan.on", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false when the relay write fails")
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.Fan.CaseFanState != fan.ModeOn {
		t.Fatalf("expected the intended state to be persisted despite the relay failure, got %q", saved.Fan.CaseFanState)
	}
	if cfg.Fan.CaseFanState != fan.ModeOn {
		t.Fatalf("expected cfg to reflect the persisted state despite the relay failure, got %q", cfg.Fan.CaseFanState)
	}
}
