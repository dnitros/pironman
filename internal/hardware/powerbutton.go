package hardware

import "time"

// KeyPowerCode is the Linux input-event code for KEY_POWER (linux/input-event-codes.h).
const KeyPowerCode = 116

type PowerButtonEvent struct {
	Pressed bool
	At      time.Time
}

type PowerButtonWatcher interface {
	Next() (PowerButtonEvent, error)
	Close() error
}
