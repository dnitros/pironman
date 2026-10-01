package rgb

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dnitros/pironman/internal/hardware"
)

const (
	StyleSolid     = "solid"
	StyleBreathing = "breathing"
)

var validStyles = []string{StyleSolid, StyleBreathing}

func ValidateStyle(name string) error {
	if slices.Contains(validStyles, name) {
		return nil
	}
	return fmt.Errorf("invalid style %q: want one of %s", name, strings.Join(validStyles, ", "))
}

func ValidateSpeed(percent int) error {
	if percent < 0 || percent > 100 {
		return fmt.Errorf("invalid speed %d: want 0-100", percent)
	}
	return nil
}

func isAnimated(style string) bool {
	return style == StyleBreathing
}

// breathingSteps is the length of the triangular fade cycle, matching
// sunfounder/pm_auto's ws2812.py breathing effect.
const breathingSteps = 200

func BreathingFrame(frame int, r, g, b byte, brightnessPercent, numLEDs int) []hardware.Color {
	pos := frame % breathingSteps
	half := breathingSteps / 2

	levelPercent := pos * 99 / half
	if pos >= half {
		levelPercent = (breathingSteps - pos) * 99 / half
	}

	scaledPercent := brightnessPercent * levelPercent / 100
	color := hardware.Color{R: scale(r, scaledPercent), G: scale(g, scaledPercent), B: scale(b, scaledPercent)}

	pixels := make([]hardware.Color, numLEDs)
	for i := range pixels {
		pixels[i] = color
	}
	return pixels
}

const (
	breathingDelayMax = 100 * time.Millisecond // speed 0 (slowest)
	breathingDelayMin = 1 * time.Millisecond   // speed 100 (fastest)
)

func BreathingDelay(speedPercent int) time.Duration {
	span := breathingDelayMax - breathingDelayMin
	return breathingDelayMax - span*time.Duration(speedPercent)/100
}

// animationFrame computes one frame for the given animated style, driven by
// the Store's self-paced animation loop.
func animationFrame(style string, frame int, r, g, b byte, brightnessPercent, speedPercent, numLEDs int) ([]hardware.Color, time.Duration, error) {
	switch style {
	case StyleBreathing:
		return BreathingFrame(frame, r, g, b, brightnessPercent, numLEDs), BreathingDelay(speedPercent), nil
	default:
		return nil, 0, fmt.Errorf("rgb: unknown animated style %q", style)
	}
}
