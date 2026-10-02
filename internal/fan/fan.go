package fan

import (
	"fmt"
	"strings"
	"sync"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

const (
	ModeAlwaysOn    = "always_on"
	ModePerformance = "performance"
	ModeCool        = "cool"
	ModeBalanced    = "balanced"
	ModeQuiet       = "quiet"
)

const (
	PerformanceOnThresholdC  = 50.0
	PerformanceOffThresholdC = 45.0
	CoolOnThresholdC         = 60.0
	CoolOffThresholdC        = 55.0
	BalancedOnThresholdC     = 67.5
	BalancedOffThresholdC    = 62.5
	QuietOnThresholdC        = 75.0
	QuietOffThresholdC       = 70.0
)

type threshold struct {
	onC, offC float64
}

var curveThresholds = map[string]threshold{
	ModePerformance: {PerformanceOnThresholdC, PerformanceOffThresholdC},
	ModeCool:        {CoolOnThresholdC, CoolOffThresholdC},
	ModeBalanced:    {BalancedOnThresholdC, BalancedOffThresholdC},
	ModeQuiet:       {QuietOnThresholdC, QuietOffThresholdC},
}

var curveNames = []string{ModeAlwaysOn, ModePerformance, ModeCool, ModeBalanced, ModeQuiet}

func ValidateMode(name string) error {
	for _, n := range curveNames {
		if name == n {
			return nil
		}
	}
	return fmt.Errorf("invalid fan mode %q: want one of %s", name, strings.Join(curveNames, ", "))
}

type State struct {
	Mode    string
	RelayOn bool
}

type Machine struct {
	mu sync.Mutex

	relay hardware.Relay
	stats sysstats.Source

	mode    string
	relayOn bool
}

func NewMachine(relay hardware.Relay, stats sysstats.Source, initialMode string) (*Machine, error) {
	if err := ValidateMode(initialMode); err != nil {
		return nil, err
	}

	m := &Machine{
		relay: relay,
		stats: stats,
		mode:  initialMode,
	}
	if err := m.evaluateLocked(); err != nil {
		return nil, fmt.Errorf("apply initial fan state: %w", err)
	}
	return m, nil
}

func NewConfiguredMachine(initialMode string) (*Machine, error) {
	relay, err := hardware.NewGPIORelay(hardware.CaseFanRelayLine)
	if err != nil {
		return nil, fmt.Errorf("open case-fan relay: %w", err)
	}
	stats := sysstats.NewProcSource(sysstats.DefaultStatPath, sysstats.DefaultThermalPath, sysstats.DefaultMemInfoPath, sysstats.DefaultMountsPath)
	return NewMachine(relay, stats, initialMode)
}

func (m *Machine) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return State{Mode: m.mode, RelayOn: m.relayOn}
}

func (m *Machine) Mode(name string) error {
	if err := ValidateMode(name); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.mode = name
	return m.evaluateLocked()
}

func (m *Machine) Tick() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.evaluateLocked()
}

func (m *Machine) Off() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.setRelayLocked(false)
}

func (m *Machine) evaluateLocked() error {
	if m.mode == ModeAlwaysOn {
		return m.setRelayLocked(true)
	}

	t, ok := curveThresholds[m.mode]
	if !ok {
		return fmt.Errorf("fan: unknown mode %q", m.mode)
	}

	// ponytail: Snapshot() also reads mem/disk/network stats we don't need
	// here; add a CPU-temp-only Source method if this per-second read shows
	// up in profiling.
	snap, err := m.stats.Snapshot()
	if err != nil {
		return fmt.Errorf("read CPU temperature: %w", err)
	}

	switch {
	case snap.CPUTempC > t.onC:
		return m.setRelayLocked(true)
	case snap.CPUTempC < t.offC:
		return m.setRelayLocked(false)
	default:
		return nil
	}
}

func (m *Machine) setRelayLocked(on bool) error {
	if err := m.relay.Set(on); err != nil {
		return fmt.Errorf("set relay: %w", err)
	}
	m.relayOn = on
	return nil
}
