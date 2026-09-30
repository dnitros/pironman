package fan_test

import (
	"errors"
	"testing"

	"github.com/dnitros/pironman/internal/fan"
	"github.com/dnitros/pironman/internal/sysstats"
)

var errBoom = errors.New("boom")

type fakeRelay struct {
	onCalls, offCalls int
	setErr            error
	on                bool
}

func (f *fakeRelay) Set(on bool) error {
	if on {
		f.onCalls++
	} else {
		f.offCalls++
	}
	f.on = on
	return f.setErr
}

type fakeStats struct {
	tempC float64
	err   error
	calls int
}

func (f *fakeStats) Snapshot() (sysstats.Snapshot, error) {
	f.calls++
	return sysstats.Snapshot{CPUTempC: f.tempC}, f.err
}

func TestNewMachineOnModeEnergizesRelay(t *testing.T) {
	relay := &fakeRelay{}
	if _, err := fan.NewMachine(relay, &fakeStats{}, fan.ModeOn); err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if relay.onCalls != 1 || relay.offCalls != 0 {
		t.Fatalf("expected exactly one On, got on=%d off=%d", relay.onCalls, relay.offCalls)
	}
}

func TestNewMachineOffModeDeenergizesRelay(t *testing.T) {
	relay := &fakeRelay{}
	if _, err := fan.NewMachine(relay, &fakeStats{}, fan.ModeOff); err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if relay.offCalls != 1 || relay.onCalls != 0 {
		t.Fatalf("expected exactly one Off, got on=%d off=%d", relay.onCalls, relay.offCalls)
	}
}

func TestNewMachineAutoModeEvaluatesTemperatureImmediately(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 70}
	if _, err := fan.NewMachine(relay, stats, fan.ModeAuto); err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if stats.calls != 1 {
		t.Fatalf("expected NewMachine to read temperature once, got %d reads", stats.calls)
	}
	if relay.onCalls != 1 {
		t.Fatalf("expected the relay to turn on above the threshold, got on=%d off=%d", relay.onCalls, relay.offCalls)
	}
}

func TestNewMachineRejectsUnknownMode(t *testing.T) {
	if _, err := fan.NewMachine(&fakeRelay{}, &fakeStats{}, "bogus"); err == nil {
		t.Fatalf("expected an error for an unknown mode")
	}
}

func TestNewMachinePropagatesApplyError(t *testing.T) {
	relay := &fakeRelay{setErr: errBoom}
	if _, err := fan.NewMachine(relay, &fakeStats{}, fan.ModeOn); err == nil {
		t.Fatalf("expected an error when the initial apply fails")
	}
}

func TestOnForcesRelayRegardlessOfTemperature(t *testing.T) {
	relay := &fakeRelay{}
	m, err := fan.NewMachine(relay, &fakeStats{tempC: 10}, fan.ModeOff)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.On(); err != nil {
		t.Fatalf("On: %v", err)
	}
	state := m.State()
	if state.Mode != fan.ModeOn || !state.RelayOn {
		t.Fatalf("State() = %+v, want mode=on relay=on", state)
	}
}

func TestOffForcesRelayRegardlessOfTemperature(t *testing.T) {
	relay := &fakeRelay{}
	m, err := fan.NewMachine(relay, &fakeStats{tempC: 90}, fan.ModeOn)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.Off(); err != nil {
		t.Fatalf("Off: %v", err)
	}
	state := m.State()
	if state.Mode != fan.ModeOff || state.RelayOn {
		t.Fatalf("State() = %+v, want mode=off relay=off", state)
	}
}

func TestAutoTurnsRelayOnAboveHighThreshold(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 67.6}
	m, err := fan.NewMachine(relay, stats, fan.ModeOff)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.Auto(); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if !m.State().RelayOn {
		t.Fatalf("expected the relay to turn on above %.1f°C", fan.AutoOnThresholdC)
	}
}

func TestAutoTurnsRelayOffBelowLowThreshold(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 62.4}
	m, err := fan.NewMachine(relay, stats, fan.ModeOn)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.Auto(); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if m.State().RelayOn {
		t.Fatalf("expected the relay to turn off below %.1f°C", fan.AutoOffThresholdC)
	}
}

func TestAutoNoOpsWithinHysteresisBandWhileRelayOn(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 65}
	m, err := fan.NewMachine(relay, stats, fan.ModeOn)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	callsBefore := relay.onCalls + relay.offCalls

	if err := m.Auto(); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if !m.State().RelayOn {
		t.Fatalf("expected the relay to stay on inside the hysteresis band")
	}
	if relay.onCalls+relay.offCalls != callsBefore {
		t.Fatalf("expected no further relay writes inside the hysteresis band")
	}
}

func TestAutoNoOpsWithinHysteresisBandWhileRelayOff(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 65}
	m, err := fan.NewMachine(relay, stats, fan.ModeOff)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	callsBefore := relay.onCalls + relay.offCalls

	if err := m.Auto(); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if m.State().RelayOn {
		t.Fatalf("expected the relay to stay off inside the hysteresis band")
	}
	if relay.onCalls+relay.offCalls != callsBefore {
		t.Fatalf("expected no further relay writes inside the hysteresis band")
	}
}

func TestAutoNoOpsAtExactOnThreshold(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: fan.AutoOnThresholdC}
	m, err := fan.NewMachine(relay, stats, fan.ModeOff)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.Auto(); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if m.State().RelayOn {
		t.Fatalf("expected the relay to stay off at exactly %.1f°C (on threshold is exclusive)", fan.AutoOnThresholdC)
	}
}

func TestAutoNoOpsAtExactOffThreshold(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: fan.AutoOffThresholdC}
	m, err := fan.NewMachine(relay, stats, fan.ModeOn)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.Auto(); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if !m.State().RelayOn {
		t.Fatalf("expected the relay to stay on at exactly %.1f°C (off threshold is exclusive)", fan.AutoOffThresholdC)
	}
}

func TestAutoPropagatesStatsError(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 10}
	m, err := fan.NewMachine(relay, stats, fan.ModeOff)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	stats.err = errBoom
	if err := m.Auto(); err == nil {
		t.Fatalf("expected Auto to propagate the stats error")
	}
}

func TestNewMachinePropagatesInitialStatsErrorInAutoMode(t *testing.T) {
	relay := &fakeRelay{}
	if _, err := fan.NewMachine(relay, &fakeStats{err: errBoom}, fan.ModeAuto); err == nil {
		t.Fatalf("expected NewMachine to propagate the initial stats error in auto mode")
	}
}

func TestTickIsANoOpInOnMode(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 90}
	m, err := fan.NewMachine(relay, stats, fan.ModeOn)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	callsBefore := stats.calls

	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if stats.calls != callsBefore {
		t.Fatalf("expected Tick to not read temperature outside auto mode, got %d new reads", stats.calls-callsBefore)
	}
}

func TestTickIsANoOpInOffMode(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 10}
	m, err := fan.NewMachine(relay, stats, fan.ModeOff)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	callsBefore := stats.calls

	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if stats.calls != callsBefore {
		t.Fatalf("expected Tick to not read temperature outside auto mode, got %d new reads", stats.calls-callsBefore)
	}
}

func TestTickReevaluatesInAutoModeAsTemperatureChanges(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 10}
	m, err := fan.NewMachine(relay, stats, fan.ModeAuto)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if m.State().RelayOn {
		t.Fatalf("expected the relay to start off at 10°C")
	}

	stats.tempC = 70
	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if !m.State().RelayOn {
		t.Fatalf("expected Tick to turn the relay on once the temperature crosses the high threshold")
	}
}

func TestTickPropagatesStatsError(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 10}
	m, err := fan.NewMachine(relay, stats, fan.ModeAuto)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	stats.err = errBoom
	if err := m.Tick(); err == nil {
		t.Fatalf("expected Tick to propagate the stats error")
	}
}

func TestModePersistsAcrossSimulatedRestart(t *testing.T) {
	relay := &fakeRelay{}
	m, err := fan.NewMachine(relay, &fakeStats{tempC: 90}, fan.ModeAuto)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	restartedRelay := &fakeRelay{}
	restarted, err := fan.NewMachine(restartedRelay, &fakeStats{tempC: 90}, m.State().Mode)
	if err != nil {
		t.Fatalf("NewMachine (restart): %v", err)
	}
	if restarted.State().Mode != fan.ModeAuto {
		t.Fatalf("expected the auto mode to survive the simulated restart, got %q", restarted.State().Mode)
	}
	if !restarted.State().RelayOn {
		t.Fatalf("expected the restarted machine to re-evaluate temperature and energize the relay")
	}
}
