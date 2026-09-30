package hardware

import "time"

// PowerButtonEvent is a raw press/release transition from the power button's
// input device.
type PowerButtonEvent struct {
	Pressed bool
	At      time.Time
}

// PowerButtonWatcher discovers the power-button input device, grabs it
// exclusively, and emits raw press/release transitions.
type PowerButtonWatcher interface {
	// Next blocks until the next press/release transition or an error
	// (including one caused by Close, which unblocks a pending Next call).
	Next() (PowerButtonEvent, error)
	// Close releases the device grab and the underlying device. It unblocks
	// any goroutine waiting in Next.
	Close() error
}
