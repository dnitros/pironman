// Package powerbutton classifies raw power-button press/release transitions
// into the four CONTEXT.md press events: click, double-click, long-press,
// and long-press-released.
package powerbutton

import (
	"time"

	"github.com/dnitros/pironman/internal/clock"
)

const (
	DebounceWindow     = 250 * time.Millisecond
	LongPressThreshold = 2 * time.Second
)

type Event int

const (
	EventNone Event = iota
	EventClick
	EventDoubleClick
	EventLongPress
	EventLongPressReleased
)

type phase int

const (
	phaseIdle phase = iota
	phaseHeld
	phaseHeldResolved
	phasePendingClick
)

// Classifier is a pure state machine: it has no dependency on OLED, shutdown,
// or any other domain package, and consumes raw transitions from a
// hardware.PowerButtonWatcher via PressDown/PressUp, plus a periodic Tick for
// its time-driven checks.
type Classifier struct {
	clk    clock.Clock
	phase  phase
	downAt time.Time
}

func NewClassifier(clk clock.Clock) *Classifier {
	return &Classifier{clk: clk, phase: phaseIdle}
}

// PressDown supersedes a pending click into a double-click if it arrives
// within the debounce window of the first press-down. Otherwise it flushes
// the now-confirmed click in case Tick hasn't caught up yet, so a click is
// never dropped regardless of tick cadence, and starts tracking this press.
func (c *Classifier) PressDown() Event {
	now := c.clk.Now()

	if c.phase == phasePendingClick {
		if now.Sub(c.downAt) < DebounceWindow {
			c.phase = phaseHeldResolved
			return EventDoubleClick
		}
		c.phase = phaseHeld
		c.downAt = now
		return EventClick
	}

	c.phase = phaseHeld
	c.downAt = now
	return EventNone
}

func (c *Classifier) PressUp() Event {
	now := c.clk.Now()

	switch c.phase {
	case phaseHeldResolved:
		c.phase = phaseIdle
		return EventNone
	case phaseHeld:
		if now.Sub(c.downAt) >= LongPressThreshold {
			c.phase = phaseIdle
			return EventLongPressReleased
		}
		c.phase = phasePendingClick
		return EventNone
	default:
		return EventNone
	}
}

// Tick confirms a pending click once its debounce window expires, and
// re-fires EventLongPress on every call while the button stays held past the
// long-press threshold (level-triggered, matching the original hardware's
// firing behavior).
func (c *Classifier) Tick() Event {
	now := c.clk.Now()

	switch c.phase {
	case phasePendingClick:
		if now.Sub(c.downAt) >= DebounceWindow {
			c.phase = phaseIdle
			return EventClick
		}
	case phaseHeld:
		if now.Sub(c.downAt) >= LongPressThreshold {
			return EventLongPress
		}
	}
	return EventNone
}
