//go:build !linux

package hardware

import "fmt"

type EvdevPowerButtonWatcher struct{}

func NewEvdevPowerButtonWatcher() (*EvdevPowerButtonWatcher, error) {
	return nil, fmt.Errorf("power-button watcher: unsupported on this platform (linux required)")
}

func (w *EvdevPowerButtonWatcher) Next() (PowerButtonEvent, error) {
	return PowerButtonEvent{}, fmt.Errorf("power-button watcher: unsupported on this platform (linux required)")
}

func (w *EvdevPowerButtonWatcher) Close() error {
	return fmt.Errorf("power-button watcher: unsupported on this platform (linux required)")
}
