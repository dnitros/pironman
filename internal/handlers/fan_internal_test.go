package handlers

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/fan"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/sysstats"
)

var errBoom = errors.New("boom")

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

func newTestFanMachine(t *testing.T, mode string) (*fan.Machine, *fakeRelay) {
	t.Helper()
	relay := &fakeRelay{}
	m, err := fan.NewMachine(relay, &fakeFanStats{tempC: 50}, mode)
	if err != nil {
		t.Fatalf("fan.NewMachine: %v", err)
	}
	return m, relay
}

func TestFanHandlersModePersistsStateAndEvaluatesTemperature(t *testing.T) {
	relay := &fakeRelay{}
	machine, err := fan.NewMachine(relay, &fakeFanStats{tempC: 70}, fan.ModeQuiet)
	if err != nil {
		t.Fatalf("fan.NewMachine: %v", err)
	}
	cfg := config.Default()
	cfg.Fan.CaseFanState = fan.ModeQuiet
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, FanHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "fan.mode", map[string]any{"name": fan.ModeBalanced})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if relay.onCalls != 1 {
		t.Fatalf("expected balanced mode to energize the relay above the threshold, got %d On() calls", relay.onCalls)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.Fan.CaseFanState != fan.ModeBalanced {
		t.Fatalf("expected the saved config to have fan.case_fan_state=balanced, got %q", saved.Fan.CaseFanState)
	}
}

func TestFanHandlersModeRejectsInvalidNameWithoutTouchingRelayOrConfig(t *testing.T) {
	machine, relay := newTestFanMachine(t, fan.ModeQuiet)
	cfg := config.Default()
	cfg.Fan.CaseFanState = fan.ModeQuiet
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, FanHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))
	callsBefore := relay.onCalls + relay.offCalls

	resp, err := ipc.Send(path, "fan.mode", map[string]any{"name": "bogus"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false for an invalid mode name")
	}
	if relay.onCalls+relay.offCalls != callsBefore {
		t.Fatalf("expected the relay to never be touched for an invalid mode name")
	}
	if cfg.Fan.CaseFanState != fan.ModeQuiet {
		t.Fatalf("expected cfg to remain unchanged for an invalid mode name, got %q", cfg.Fan.CaseFanState)
	}
}

func TestFanHandlersModePropagatesPersistErrorWithoutTouchingRelay(t *testing.T) {
	machine, relay := newTestFanMachine(t, fan.ModeQuiet)
	cfg := config.Default()
	cfg.Fan.CaseFanState = fan.ModeQuiet

	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfgPath := filepath.Join(blocker, "config.yaml")

	path := startTestDaemon(t, FanHandlers(machine, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "fan.mode", map[string]any{"name": fan.ModeBalanced})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false when persisting fails")
	}
	if cfg.Fan.CaseFanState != fan.ModeQuiet {
		t.Fatalf("expected cfg to remain unchanged when persist fails, got %q", cfg.Fan.CaseFanState)
	}
	if relay.onCalls != 0 {
		t.Fatalf("expected the relay to never be touched when persisting fails, got %d On() calls", relay.onCalls)
	}
}

func TestFanHandlersModeCommitsPersistedStateEvenWhenRelayFails(t *testing.T) {
	relay := &fakeRelay{}
	m, err := fan.NewMachine(relay, &fakeFanStats{tempC: 50}, fan.ModeQuiet)
	if err != nil {
		t.Fatalf("fan.NewMachine: %v", err)
	}
	relay.setErr = errBoom

	cfg := config.Default()
	cfg.Fan.CaseFanState = fan.ModeQuiet
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, FanHandlers(m, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "fan.mode", map[string]any{"name": fan.ModeAlwaysOn})
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
	if saved.Fan.CaseFanState != fan.ModeAlwaysOn {
		t.Fatalf("expected the intended state to be persisted despite the relay failure, got %q", saved.Fan.CaseFanState)
	}
	if cfg.Fan.CaseFanState != fan.ModeAlwaysOn {
		t.Fatalf("expected cfg to reflect the persisted state despite the relay failure, got %q", cfg.Fan.CaseFanState)
	}
}
