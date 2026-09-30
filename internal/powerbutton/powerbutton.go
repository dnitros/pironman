package powerbutton

import (
	"sync"
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

type Classifier struct {
	mu sync.Mutex

	clk    clock.Clock
	phase  phase
	downAt time.Time
}

func NewClassifier(clk clock.Clock) *Classifier {
	return &Classifier{clk: clk, phase: phaseIdle}
}

func (c *Classifier) PressDown() Event {
	c.mu.Lock()
	defer c.mu.Unlock()

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
	c.mu.Lock()
	defer c.mu.Unlock()

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

func (c *Classifier) Tick() Event {
	c.mu.Lock()
	defer c.mu.Unlock()

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
