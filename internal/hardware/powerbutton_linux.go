//go:build linux

package hardware

import (
	"fmt"
	"sync"
	"time"

	"github.com/holoplot/go-evdev"
)

type EvdevPowerButtonWatcher struct {
	dev *evdev.InputDevice

	closeOnce sync.Once
	closeErr  error
}

// NewEvdevPowerButtonWatcher scans the available input devices for one that
// advertises KEY_POWER and grabs it exclusively so nothing else can read it
// concurrently.
func NewEvdevPowerButtonWatcher() (*EvdevPowerButtonWatcher, error) {
	dev, err := findPowerButtonDevice()
	if err != nil {
		return nil, err
	}
	if err := dev.Grab(); err != nil {
		_ = dev.Close()
		return nil, fmt.Errorf("grab power-button device: %w", err)
	}
	return &EvdevPowerButtonWatcher{dev: dev}, nil
}

func findPowerButtonDevice() (*evdev.InputDevice, error) {
	paths, err := evdev.ListDevicePaths()
	if err != nil {
		return nil, fmt.Errorf("list input devices: %w", err)
	}

	for _, p := range paths {
		dev, err := evdev.Open(p.Path)
		if err != nil {
			continue
		}
		if hasPowerKey(dev.CapableEvents(evdev.EV_KEY)) {
			return dev, nil
		}
		_ = dev.Close()
	}

	return nil, fmt.Errorf("power-button input device not found (no device advertises KEY_POWER)")
}

func hasPowerKey(codes []evdev.EvCode) bool {
	for _, c := range codes {
		if c == evdev.KEY_POWER {
			return true
		}
	}
	return false
}

// Next blocks on the device until a KEY_POWER press or release, skipping
// unrelated events and autorepeat (Value == 2).
func (w *EvdevPowerButtonWatcher) Next() (PowerButtonEvent, error) {
	for {
		ev, err := w.dev.ReadOne()
		if err != nil {
			return PowerButtonEvent{}, fmt.Errorf("read power-button event: %w", err)
		}
		if ev.Type != evdev.EV_KEY || ev.Code != evdev.KEY_POWER {
			continue
		}

		at := time.Unix(int64(ev.Time.Sec), int64(ev.Time.Usec)*1000)
		switch ev.Value {
		case 1:
			return PowerButtonEvent{Pressed: true, At: at}, nil
		case 0:
			return PowerButtonEvent{Pressed: false, At: at}, nil
		default:
			continue
		}
	}
}

// Close is idempotent: the daemon's shutdown sequence may invoke it more
// than once (once directly, once as a deferred cleanup), and only the first
// call should touch the device.
func (w *EvdevPowerButtonWatcher) Close() error {
	w.closeOnce.Do(func() {
		if err := w.dev.Ungrab(); err != nil {
			_ = w.dev.Close()
			w.closeErr = fmt.Errorf("ungrab power-button device: %w", err)
			return
		}
		w.closeErr = w.dev.Close()
	})
	return w.closeErr
}
