//go:build linux

package hardware

import (
	"fmt"

	"github.com/warthog618/go-gpiocdev"
)

type GPIORelay struct {
	line *gpiocdev.Line
}

func NewGPIORelay(name string) (*GPIORelay, error) {
	chip, offset, err := gpiocdev.FindLine(name)
	if err != nil {
		return nil, fmt.Errorf("find GPIO line %q: %w", name, err)
	}
	line, err := gpiocdev.RequestLine(chip, offset, gpiocdev.AsOutput(0), gpiocdev.WithConsumer("pironman"))
	if err != nil {
		return nil, fmt.Errorf("open relay line %q (chip %s offset %d): %w", name, chip, offset, err)
	}
	return &GPIORelay{line: line}, nil
}

func (r *GPIORelay) Set(on bool) error {
	v := 0
	if on {
		v = 1
	}
	if err := r.line.SetValue(v); err != nil {
		return fmt.Errorf("set relay line: %w", err)
	}
	return nil
}
