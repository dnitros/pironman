package rgb

import (
	"fmt"
	"strings"
	"time"

	"github.com/dnitros/pironman/internal/hardware"
)

const (
	StyleSolid     = "solid"
	StyleBreathing = "breathing"
)

// frameFunc computes one animation frame and the delay before the next.
type frameFunc func(frame int, r, g, b byte, brightnessPercent, speedPercent, numLEDs int) ([]hardware.Color, time.Duration)

// styleDef is the single source of truth for a style's name, whether it
// animates, and how it computes frames — name validation, the
// solid/animated split, and frame dispatch all read from this list instead
// of keeping separate, driftable copies of "which styles exist."
type styleDef struct {
	name  string
	frame frameFunc // nil for the non-animated solid style
}

var styleDefs = []styleDef{
	{name: StyleSolid},
	{name: StyleBreathing, frame: func(frame int, r, g, b byte, brightnessPercent, speedPercent, numLEDs int) ([]hardware.Color, time.Duration) {
		return BreathingFrame(frame, r, g, b, brightnessPercent, numLEDs), BreathingDelay(speedPercent)
	}},
}

func lookupStyle(name string) (styleDef, bool) {
	for _, d := range styleDefs {
		if d.name == name {
			return d, true
		}
	}
	return styleDef{}, false
}

func ValidateStyle(name string) error {
	if _, ok := lookupStyle(name); ok {
		return nil
	}
	names := make([]string, len(styleDefs))
	for i, d := range styleDefs {
		names[i] = d.name
	}
	return fmt.Errorf("invalid style %q: want one of %s", name, strings.Join(names, ", "))
}

func ValidateSpeed(percent int) error {
	if percent < 0 || percent > 100 {
		return fmt.Errorf("invalid speed %d: want 0-100", percent)
	}
	return nil
}

func isAnimated(style string) bool {
	d, ok := lookupStyle(style)
	return ok && d.frame != nil
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

	// ponytail: allocates one small (NumLEDs-length) slice per frame; skip a
	// reused buffer unless profiling shows animation GC pressure, since
	// encodeFrame already allocates a larger buffer per frame regardless.
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
	d, ok := lookupStyle(style)
	if !ok || d.frame == nil {
		return nil, 0, fmt.Errorf("rgb: unknown animated style %q", style)
	}
	pixels, delay := d.frame(frame, r, g, b, brightnessPercent, speedPercent, numLEDs)
	return pixels, delay, nil
}
