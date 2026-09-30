package hardware

import "time"

type PowerButtonEvent struct {
	Pressed bool
	At      time.Time
}

type PowerButtonWatcher interface {
	Next() (PowerButtonEvent, error)
	Close() error
}
