//go:build linux

package hardware

import (
	"fmt"

	"github.com/warthog618/go-gpiocdev"
)

// caseFanRelayChips are tried in order for RP1 compatibility across
// Raspberry Pi OS versions.
var caseFanRelayChips = []string{"gpiochip4", "gpiochip0", "gpiochip1"}

type GPIORelay struct {
	line *gpiocdev.Line
}

func NewGPIORelay(offset int) (*GPIORelay, error) {
	var (
		line *gpiocdev.Line
		err  error
	)
	for _, chip := range caseFanRelayChips {
		line, err = gpiocdev.RequestLine(chip, offset, gpiocdev.AsOutput(0))
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("open relay line %d on %v: %w", offset, caseFanRelayChips, err)
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
