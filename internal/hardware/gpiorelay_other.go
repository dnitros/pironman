//go:build !linux

package hardware

import "fmt"

type GPIORelay struct{}

func NewGPIORelay(name string) (*GPIORelay, error) {
	return nil, fmt.Errorf("case-fan relay: unsupported on this platform (linux required)")
}

func (r *GPIORelay) Set(on bool) error {
	return fmt.Errorf("case-fan relay: unsupported on this platform (linux required)")
}
