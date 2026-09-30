package fan

import (
	"fmt"
	"sync"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

const (
	ModeOn   = "on"
	ModeOff  = "off"
	ModeAuto = "auto"
)

const (
	AutoOnThresholdC  = 67.5
	AutoOffThresholdC = 62.5
)

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
	if !validMode(initialMode) {
		return nil, fmt.Errorf("fan: invalid mode %q", initialMode)
	}

	m := &Machine{
		relay: relay,
		stats: stats,
		mode:  initialMode,
	}
	if err := m.applyLocked(); err != nil {
		return nil, fmt.Errorf("apply initial fan state: %w", err)
	}
	return m, nil
}

func NewMachineFromConfig(cfg config.Config) (*Machine, error) {
	relay, err := hardware.NewGPIORelay(hardware.CaseFanRelayLine)
	if err != nil {
		return nil, fmt.Errorf("open case-fan relay: %w", err)
	}
	stats := sysstats.NewProcSource(sysstats.DefaultStatPath, sysstats.DefaultThermalPath, sysstats.DefaultMemInfoPath, sysstats.DefaultMountsPath)
	return NewMachine(relay, stats, cfg.Fan.CaseFanState)
}

func (m *Machine) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return State{Mode: m.mode, RelayOn: m.relayOn}
}

func (m *Machine) On() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mode = ModeOn
	return m.setRelayLocked(true)
}

func (m *Machine) Off() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mode = ModeOff
	return m.setRelayLocked(false)
}

func (m *Machine) Auto() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mode = ModeAuto
	return m.evaluateLocked()
}

func (m *Machine) Tick() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mode != ModeAuto {
		return nil
	}
	return m.evaluateLocked()
}

func (m *Machine) applyLocked() error {
	switch m.mode {
	case ModeOn:
		return m.setRelayLocked(true)
	case ModeOff:
		return m.setRelayLocked(false)
	case ModeAuto:
		return m.evaluateLocked()
	default:
		return fmt.Errorf("fan: unknown mode %q", m.mode)
	}
}

func (m *Machine) evaluateLocked() error {
	// ponytail: Snapshot() also reads mem/disk/network stats we don't need
	// here; add a CPU-temp-only Source method if this per-second read shows
	// up in profiling.
	snap, err := m.stats.Snapshot()
	if err != nil {
		return fmt.Errorf("read CPU temperature: %w", err)
	}

	switch {
	case snap.CPUTempC > AutoOnThresholdC:
		return m.setRelayLocked(true)
	case snap.CPUTempC < AutoOffThresholdC:
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

func validMode(mode string) bool {
	switch mode {
	case ModeOn, ModeOff, ModeAuto:
		return true
	default:
		return false
	}
}
