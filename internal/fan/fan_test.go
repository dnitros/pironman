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

func TestNewMachineRejectsUnknownMode(t *testing.T) {
	if _, err := fan.NewMachine(&fakeRelay{}, &fakeStats{}, "bogus"); err == nil {
		t.Fatalf("expected an error for an unknown mode")
	}
}

func TestNewMachineRejectsLegacyAutoMode(t *testing.T) {
	if _, err := fan.NewMachine(&fakeRelay{}, &fakeStats{}, "auto"); err == nil {
		t.Fatalf("expected an error for the removed legacy auto mode")
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

func TestModeRejectsUnknownName(t *testing.T) {
	m, err := fan.NewMachine(&fakeRelay{}, &fakeStats{}, fan.ModeOff)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if err := m.Mode("bogus"); err == nil {
		t.Fatalf("expected an error for an unknown mode name")
	}
}

func TestModeRejectsOnAndOff(t *testing.T) {
	m, err := fan.NewMachine(&fakeRelay{}, &fakeStats{}, fan.ModeOff)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if err := m.Mode(fan.ModeOn); err == nil {
		t.Fatalf("expected Mode to reject %q; use On() instead", fan.ModeOn)
	}
	if err := m.Mode(fan.ModeOff); err == nil {
		t.Fatalf("expected Mode to reject %q; use Off() instead", fan.ModeOff)
	}
}

func TestModeErrorListsTheFiveCurveNames(t *testing.T) {
	m, err := fan.NewMachine(&fakeRelay{}, &fakeStats{}, fan.ModeOff)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	err = m.Mode("bogus")
	if err == nil {
		t.Fatalf("expected an error")
	}
	for _, name := range []string{fan.ModeAlwaysOn, fan.ModePerformance, fan.ModeCool, fan.ModeBalanced, fan.ModeQuiet} {
		if !containsSubstring(err.Error(), name) {
			t.Fatalf("expected error %q to mention valid mode %q", err, name)
		}
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAlwaysOnKeepsRelayOnRegardlessOfTemperatureAndSkipsStatsRead(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: -40}
	m, err := fan.NewMachine(relay, stats, fan.ModeOff)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	if err := m.Mode(fan.ModeAlwaysOn); err != nil {
		t.Fatalf("Mode: %v", err)
	}
	if !m.State().RelayOn {
		t.Fatalf("expected always_on to energize the relay regardless of temperature")
	}
	if stats.calls != 0 {
		t.Fatalf("expected always_on to never read temperature, got %d reads", stats.calls)
	}

	if err := m.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if stats.calls != 0 {
		t.Fatalf("expected Tick in always_on mode to never read temperature, got %d reads", stats.calls)
	}
}

func TestPerformanceCurveThresholds(t *testing.T) {
	relay := &fakeRelay{}
	m, err := fan.NewMachine(relay, &fakeStats{tempC: 50.1}, fan.ModePerformance)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if !m.State().RelayOn {
		t.Fatalf("expected performance to turn on above 50°C")
	}

	stats2 := &fakeStats{tempC: 44.9}
	m2, err := fan.NewMachine(relay, stats2, fan.ModePerformance)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if err := m2.Mode(fan.ModePerformance); err != nil {
		t.Fatalf("Mode: %v", err)
	}
	if m2.State().RelayOn {
		t.Fatalf("expected performance to turn off below 45°C")
	}
}

func TestCoolCurveThresholds(t *testing.T) {
	relay := &fakeRelay{}
	m, err := fan.NewMachine(relay, &fakeStats{tempC: 60.1}, fan.ModeCool)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if !m.State().RelayOn {
		t.Fatalf("expected cool to turn on above 60°C")
	}

	m2, err := fan.NewMachine(relay, &fakeStats{tempC: 54.9}, fan.ModeCool)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if m2.State().RelayOn {
		t.Fatalf("expected cool to turn off below 55°C")
	}
}

func TestBalancedCurveThresholds(t *testing.T) {
	relay := &fakeRelay{}
	m, err := fan.NewMachine(relay, &fakeStats{tempC: 67.6}, fan.ModeBalanced)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if !m.State().RelayOn {
		t.Fatalf("expected balanced to turn on above 67.5°C")
	}

	m2, err := fan.NewMachine(relay, &fakeStats{tempC: 62.4}, fan.ModeBalanced)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if m2.State().RelayOn {
		t.Fatalf("expected balanced to turn off below 62.5°C")
	}
}

func TestQuietCurveThresholds(t *testing.T) {
	relay := &fakeRelay{}
	m, err := fan.NewMachine(relay, &fakeStats{tempC: 75.1}, fan.ModeQuiet)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if !m.State().RelayOn {
		t.Fatalf("expected quiet to turn on above 75°C")
	}

	m2, err := fan.NewMachine(relay, &fakeStats{tempC: 69.9}, fan.ModeQuiet)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	if m2.State().RelayOn {
		t.Fatalf("expected quiet to turn off below 70°C")
	}
}

func TestCurveNoOpsWithinHysteresisBand(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 65}
	m, err := fan.NewMachine(relay, stats, fan.ModeOn)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}
	callsBefore := relay.onCalls + relay.offCalls

	if err := m.Mode(fan.ModeBalanced); err != nil {
		t.Fatalf("Mode: %v", err)
	}
	if !m.State().RelayOn {
		t.Fatalf("expected the relay to stay on inside the hysteresis band")
	}
	if relay.onCalls+relay.offCalls != callsBefore {
		t.Fatalf("expected no further relay writes inside the hysteresis band")
	}
}

func TestModePropagatesStatsError(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 10}
	m, err := fan.NewMachine(relay, stats, fan.ModeOff)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	stats.err = errBoom
	if err := m.Mode(fan.ModeBalanced); err == nil {
		t.Fatalf("expected Mode to propagate the stats error")
	}
}

func TestNewMachinePropagatesInitialStatsErrorInCurveMode(t *testing.T) {
	relay := &fakeRelay{}
	if _, err := fan.NewMachine(relay, &fakeStats{err: errBoom}, fan.ModeBalanced); err == nil {
		t.Fatalf("expected NewMachine to propagate the initial stats error in a curve mode")
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
		t.Fatalf("expected Tick to not read temperature outside curve modes, got %d new reads", stats.calls-callsBefore)
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
		t.Fatalf("expected Tick to not read temperature outside curve modes, got %d new reads", stats.calls-callsBefore)
	}
}

func TestTickReevaluatesInCurveModeAsTemperatureChanges(t *testing.T) {
	relay := &fakeRelay{}
	stats := &fakeStats{tempC: 10}
	m, err := fan.NewMachine(relay, stats, fan.ModeBalanced)
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
	m, err := fan.NewMachine(relay, stats, fan.ModeBalanced)
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
	m, err := fan.NewMachine(relay, &fakeStats{tempC: 90}, fan.ModeBalanced)
	if err != nil {
		t.Fatalf("NewMachine: %v", err)
	}

	restartedRelay := &fakeRelay{}
	restarted, err := fan.NewMachine(restartedRelay, &fakeStats{tempC: 90}, m.State().Mode)
	if err != nil {
		t.Fatalf("NewMachine (restart): %v", err)
	}
	if restarted.State().Mode != fan.ModeBalanced {
		t.Fatalf("expected the balanced mode to survive the simulated restart, got %q", restarted.State().Mode)
	}
	if !restarted.State().RelayOn {
		t.Fatalf("expected the restarted machine to re-evaluate temperature and energize the relay")
	}
}
