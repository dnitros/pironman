package rgb

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dnitros/pironman/internal/hardware"
)

var sleep = time.Sleep

type State struct {
	Enabled    bool
	Color      string
	Brightness int
	Style      string
	Speed      int
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

	animCancel context.CancelFunc
	animDone   chan struct{}
}

func NewStore(strip hardware.WS2812Strip, initial State) (*Store, error) {
	s := &Store{strip: strip, state: initial}
	if err := s.apply(); err != nil {
		return nil, fmt.Errorf("apply initial RGB state: %w", err)
	}
	return s, nil
}

func NewConfiguredStore(enabled bool, color string, brightness int, style string, speed int) (*Store, error) {
	r, g, b, err := ScaledColor(color, brightness)
	if err != nil {
		return nil, fmt.Errorf("parse configured RGB color/brightness: %w", err)
	}
	if err := ValidateStyle(style); err != nil {
		return nil, fmt.Errorf("configured RGB style: %w", err)
	}
	if err := ValidateSpeed(speed); err != nil {
		return nil, fmt.Errorf("configured RGB speed: %w", err)
	}
	strip, err := hardware.NewSPIWS2812(hardware.SPIPort, hardware.NumLEDs, r, g, b)
	if err != nil {
		return nil, fmt.Errorf("open WS2812 strip: %w", err)
	}
	return NewStore(strip, State{Enabled: enabled, Color: color, Brightness: brightness, Style: style, Speed: speed})
}

func (s *Store) apply() error {
	if !s.state.Enabled {
		return s.strip.Off()
	}
	if isAnimated(s.state.Style) {
		s.startAnimationLocked()
		return nil
	}
	return s.strip.On()
}

func (s *Store) On() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if isAnimated(s.state.Style) {
		s.state.Enabled = true
		s.startAnimationLocked()
		return s.state, nil
	}

	if err := s.strip.On(); err != nil {
		return s.state, fmt.Errorf("turn RGB strip on: %w", err)
	}
	s.state.Enabled = true
	return s.state, nil
}

func (s *Store) Off() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopAndWaitLocked()
	if err := s.strip.Off(); err != nil {
		return s.state, fmt.Errorf("turn RGB strip off: %w", err)
	}
	s.state.Enabled = false
	return s.state, nil
}

func (s *Store) SetStyle(name string, speed *int) (State, error) {
	if err := ValidateStyle(name); err != nil {
		return State{}, err
	}
	if speed != nil {
		if err := ValidateSpeed(*speed); err != nil {
			return State{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	prevStyle, prevSpeed := s.state.Style, s.state.Speed
	wasAnimated := isAnimated(prevStyle)

	s.state.Style = name
	if speed != nil {
		s.state.Speed = *speed
	}
	nowAnimated := isAnimated(s.state.Style)

	if !s.state.Enabled || wasAnimated == nowAnimated {
		return s.state, nil
	}

	if nowAnimated {
		s.startAnimationLocked()
		return s.state, nil
	}

	s.stopAndWaitLocked()
	if err := s.applyScaled(s.state.Color, s.state.Brightness); err != nil {
		s.state.Style, s.state.Speed = prevStyle, prevSpeed
		s.startAnimationLocked()
		return s.state, err
	}
	return s.state, nil
}

func (s *Store) startAnimationLocked() {
	if s.animCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.animCancel = cancel
	s.animDone = done
	go s.runAnimation(ctx, done)
}

// Unlocking here is safe only because opMu serializes every RGB handler.
func (s *Store) stopAndWaitLocked() {
	if s.animCancel == nil {
		return
	}
	cancel := s.animCancel
	done := s.animDone
	s.animCancel = nil
	s.animDone = nil

	cancel()
	s.mu.Unlock()
	<-done
	s.mu.Lock()
}

func (s *Store) runAnimation(ctx context.Context, done chan<- struct{}) {
	defer close(done)

	for frame := 0; ; frame++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.mu.Lock()
		style, hexColor, brightness, speed := s.state.Style, s.state.Color, s.state.Brightness, s.state.Speed
		s.mu.Unlock()

		r, g, b, err := ParseColor(hexColor)
		if err != nil {
			log.Printf("rgb: animation stopped, invalid configured color %q: %v", hexColor, err)
			return
		}

		pixels, delay, err := animationFrame(style, frame, r, g, b, brightness, speed, hardware.NumLEDs)
		if err != nil {
			log.Printf("rgb: %v", err)
			return
		}

		if err := s.strip.WriteFrame(pixels); err != nil {
			log.Printf("rgb: animation frame write failed: %v", err)
		}

		sleep(delay)
	}
}

func (s *Store) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
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

func (s *Store) SetBrightness(percent int) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.applyScaled(s.state.Color, percent); err != nil {
		return s.state, err
	}
	s.state.Brightness = percent
	return s.state, nil
}

func (s *Store) applyScaled(hex string, percent int) error {
	r, g, b, err := ScaledColor(hex, percent)
	if err != nil {
		return err
	}

	s.strip.SetColor(r, g, b)
	if !s.state.Enabled || isAnimated(s.state.Style) {
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
