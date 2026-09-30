//go:build !linux

package hardware

import "fmt"

// GPIORelay's go-gpiocdev backend requires the Linux GPIO character-device
// ABI, so it can only be used when built for linux.
type GPIORelay struct{}

func NewGPIORelay(offset int) (*GPIORelay, error) {
	return nil, fmt.Errorf("case-fan relay: unsupported on this platform (linux required)")
}

func (r *GPIORelay) Set(on bool) error {
	return fmt.Errorf("case-fan relay: unsupported on this platform (linux required)")
}
