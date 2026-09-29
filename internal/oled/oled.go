// Package oled drives the OLED page state machine: awake/asleep, the current
// page, and per-page content-scroll position.
package oled

import (
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

const (
	PageMix         = "mix"
	PagePerformance = "performance"
	PageIPs         = "ips"
	PageDisk        = "disk"
)

// Clock abstracts time.Now so tests can drive the sleep-timeout and
// content-scroll timers without a real wall-clock wait.
type Clock interface {
	Now() time.Time
}

// RealClock is the production Clock, backed by the wall clock.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

type State struct {
	Awake bool
	Page  string
}

type Machine struct {
	mu sync.Mutex

	display hardware.SSD1306Display
	stats   sysstats.Source
	clock   Clock

	pages   []string
	pageIdx int

	sleepTimeout   time.Duration
	scrollInterval time.Duration

	awake        bool
	lastActivity time.Time
	lastScroll   time.Time
	scrollIdx    int
}

// NewMachine builds the state machine and applies initialAwake once,
// mirroring rgb.NewStore's apply-on-construct pattern.
func NewMachine(display hardware.SSD1306Display, stats sysstats.Source, clock Clock, pages []string, sleepTimeout, scrollInterval time.Duration, initialAwake bool) (*Machine, error) {
	if len(pages) == 0 {
		return nil, fmt.Errorf("oled: page order must not be empty")
	}

	now := clock.Now()
	m := &Machine{
		display:        display,
		stats:          stats,
		clock:          clock,
		pages:          pages,
		sleepTimeout:   sleepTimeout,
		scrollInterval: scrollInterval,
		awake:          initialAwake,
		lastActivity:   now,
		lastScroll:     now,
	}
	if err := m.renderLocked(); err != nil {
		return nil, fmt.Errorf("apply initial OLED state: %w", err)
	}
	return m, nil
}

func (m *Machine) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return State{Awake: m.awake, Page: m.pages[m.pageIdx]}
}

// On wakes the display and jumps straight to the mix page.
func (m *Machine) On() error {
	return m.SetPage(PageMix)
}

// Off blanks the display.
func (m *Machine) Off() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.awake = false
	return m.renderLocked()
}

// Advance wakes the display if asleep, or moves to the next page if already
// awake — never both in the same call.
func (m *Machine) Advance() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.awake {
		m.awake = true
		m.resetScrollLocked()
		m.lastActivity = m.clock.Now()
		return m.renderLocked()
	}

	m.pageIdx = (m.pageIdx + 1) % len(m.pages)
	m.resetScrollLocked()
	m.lastActivity = m.clock.Now()
	return m.renderLocked()
}

// Previous moves to the previous page, but only while already awake; it is a
// no-op while asleep.
func (m *Machine) Previous() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.awake {
		return nil
	}

	m.pageIdx = (m.pageIdx - 1 + len(m.pages)) % len(m.pages)
	m.resetScrollLocked()
	m.lastActivity = m.clock.Now()
	return m.renderLocked()
}

// SetPage always wakes the display and jumps directly to name, regardless of
// current state.
func (m *Machine) SetPage(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx, ok := indexOf(m.pages, name)
	if !ok {
		return fmt.Errorf("oled: unknown page %q", name)
	}

	m.pageIdx = idx
	m.awake = true
	m.resetScrollLocked()
	m.lastActivity = m.clock.Now()
	return m.renderLocked()
}

// Tick drives the 1-second refresh, content-scroll, and sleep-timeout. It is
// a no-op while asleep.
func (m *Machine) Tick() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.awake {
		return nil
	}

	now := m.clock.Now()
	if now.Sub(m.lastActivity) >= m.sleepTimeout {
		m.awake = false
		return m.renderLocked()
	}

	if now.Sub(m.lastScroll) >= m.scrollInterval {
		m.scrollIdx++
		m.lastScroll = now
	}

	return m.renderLocked()
}

func (m *Machine) resetScrollLocked() {
	m.scrollIdx = 0
	m.lastScroll = m.clock.Now()
}

func indexOf(pages []string, name string) (int, bool) {
	for i, p := range pages {
		if p == name {
			return i, true
		}
	}
	return 0, false
}

func (m *Machine) renderLocked() error {
	if !m.awake {
		return m.display.Draw(image.NewGray(image.Rect(0, 0, hardware.SSD1306Width, hardware.SSD1306Height)))
	}

	lines, err := m.pageLinesLocked()
	if err != nil {
		return err
	}
	return m.display.Draw(renderLines(lines))
}

func (m *Machine) pageLinesLocked() ([]string, error) {
	page := m.pages[m.pageIdx]
	if page != PageMix {
		return []string{page, "(coming soon)"}, nil
	}

	snap, err := m.stats.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("read stats: %w", err)
	}
	return mixLines(snap, m.scrollIdx), nil
}
