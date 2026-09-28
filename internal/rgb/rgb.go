// Package rgb holds RGB strip domain logic against a hardware.WS2812Strip
// interface, with no direct SPI/WS2812 access.
package rgb

import (
	"fmt"
	"sync"

	"github.com/dnitros/pironman/internal/hardware"
)

// State is the RGB strip's persisted on/off state.
type State struct {
	Enabled bool
}

// Store is the daemon's mutex-guarded in-process RGB state. It applies the
// initial state to the strip once on construction, and again on every
// state-mutating command.
type Store struct {
	mu    sync.Mutex
	strip hardware.WS2812Strip
	state State
}

// NewStore applies initial to strip once and returns a ready Store.
func NewStore(strip hardware.WS2812Strip, initial State) (*Store, error) {
	s := &Store{strip: strip, state: initial}
	if err := s.apply(); err != nil {
		return nil, fmt.Errorf("apply initial RGB state: %w", err)
	}
	return s, nil
}

func (s *Store) apply() error {
	if s.state.Enabled {
		return s.strip.On()
	}
	return s.strip.Off()
}

// On turns the strip on and returns the resulting state.
func (s *Store) On() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.strip.On(); err != nil {
		return s.state, fmt.Errorf("turn RGB strip on: %w", err)
	}
	s.state.Enabled = true
	return s.state, nil
}

// Off turns the strip off and returns the resulting state.
func (s *Store) Off() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.strip.Off(); err != nil {
		return s.state, fmt.Errorf("turn RGB strip off: %w", err)
	}
	s.state.Enabled = false
	return s.state, nil
}

// Enabled reports the current state.
func (s *Store) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Enabled
}
