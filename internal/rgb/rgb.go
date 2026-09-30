package rgb

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/hardware"
)

type State struct {
	Enabled    bool
	Color      string
	Brightness int
}

func ParseColor(hexColor string) (r, g, b byte, err error) {
	if len(hexColor) != 7 || hexColor[0] != '#' {
		return 0, 0, 0, fmt.Errorf("invalid color %q: want a 6-digit hex color like #ff00ff", hexColor)
	}
	raw, err := hex.DecodeString(hexColor[1:])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid color %q: want a 6-digit hex color like #ff00ff", hexColor)
	}
	return raw[0], raw[1], raw[2], nil
}

func ValidateBrightness(percent int) error {
	if percent < 0 || percent > 100 {
		return fmt.Errorf("invalid brightness %d: want 0-100", percent)
	}
	return nil
}

func ScaledColor(hexColor string, percent int) (r, g, b byte, err error) {
	r, g, b, err = ParseColor(hexColor)
	if err != nil {
		return 0, 0, 0, err
	}
	if err := ValidateBrightness(percent); err != nil {
		return 0, 0, 0, err
	}
	return scale(r, percent), scale(g, percent), scale(b, percent), nil
}

func scale(c byte, percent int) byte {
	return byte(int(c) * percent / 100)
}

type Store struct {
	mu    sync.Mutex
	strip hardware.WS2812Strip
	state State
}

func NewStore(strip hardware.WS2812Strip, initial State) (*Store, error) {
	s := &Store{strip: strip, state: initial}
	if err := s.apply(); err != nil {
		return nil, fmt.Errorf("apply initial RGB state: %w", err)
	}
	return s, nil
}

func NewStoreFromConfig(cfg config.Config) (*Store, error) {
	r, g, b, err := ScaledColor(cfg.RGB.Color, cfg.RGB.Brightness)
	if err != nil {
		return nil, fmt.Errorf("parse configured RGB color/brightness: %w", err)
	}
	strip, err := hardware.NewSPIWS2812(hardware.SPIPort, hardware.NumLEDs, r, g, b)
	if err != nil {
		return nil, fmt.Errorf("open WS2812 strip: %w", err)
	}
	return NewStore(strip, State{Enabled: cfg.RGB.Enabled, Color: cfg.RGB.Color, Brightness: cfg.RGB.Brightness})
}

func (s *Store) apply() error {
	if s.state.Enabled {
		return s.strip.On()
	}
	return s.strip.Off()
}

func (s *Store) On() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.strip.On(); err != nil {
		return s.state, fmt.Errorf("turn RGB strip on: %w", err)
	}
	s.state.Enabled = true
	return s.state, nil
}

func (s *Store) Off() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.strip.Off(); err != nil {
		return s.state, fmt.Errorf("turn RGB strip off: %w", err)
	}
	s.state.Enabled = false
	return s.state, nil
}

func (s *Store) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Store) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Enabled
}

func (s *Store) SetColor(hex string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.applyScaled(hex, s.state.Brightness); err != nil {
		return s.state, err
	}
	s.state.Color = hex
	return s.state, nil
}

func (s *Store) Color() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Color
}

func (s *Store) SetBrightness(percent int) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.applyScaled(s.state.Color, percent); err != nil {
		return s.state, err
	}
	s.state.Brightness = percent
	return s.state, nil
}

func (s *Store) Brightness() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.Brightness
}

func (s *Store) applyScaled(hex string, percent int) error {
	r, g, b, err := ScaledColor(hex, percent)
	if err != nil {
		return err
	}

	s.strip.SetColor(r, g, b)
	if !s.state.Enabled {
		return nil
	}

	if err := s.strip.On(); err != nil {
		if prevR, prevG, prevB, perr := ScaledColor(s.state.Color, s.state.Brightness); perr == nil {
			s.strip.SetColor(prevR, prevG, prevB)
		}
		return fmt.Errorf("apply RGB strip: %w", err)
	}
	return nil
}
