package rgb

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/dnitros/pironman/internal/hardware"
)

const (
	StyleSolid          = "solid"
	StyleBreathing      = "breathing"
	StyleFlow           = "flow"
	StyleFlowReverse    = "flow_reverse"
	StyleRainbow        = "rainbow"
	StyleRainbowReverse = "rainbow_reverse"
)

type frameFunc func(frame int, r, g, b byte, brightnessPercent, speedPercent, numLEDs int) ([]hardware.Color, time.Duration)

type styleDef struct {
	name  string
	frame frameFunc
}

var styleDefs = []styleDef{
	{name: StyleSolid},
	{name: StyleBreathing, frame: func(frame int, r, g, b byte, brightnessPercent, speedPercent, numLEDs int) ([]hardware.Color, time.Duration) {
		return BreathingFrame(frame, r, g, b, brightnessPercent, numLEDs), BreathingDelay(speedPercent)
	}},
	{name: StyleFlow, frame: func(frame int, r, g, b byte, brightnessPercent, speedPercent, numLEDs int) ([]hardware.Color, time.Duration) {
		return FlowFrame(frame, r, g, b, brightnessPercent, numLEDs, false), FlowDelay(speedPercent)
	}},
	{name: StyleFlowReverse, frame: func(frame int, r, g, b byte, brightnessPercent, speedPercent, numLEDs int) ([]hardware.Color, time.Duration) {
		return FlowFrame(frame, r, g, b, brightnessPercent, numLEDs, true), FlowDelay(speedPercent)
	}},
	{name: StyleRainbow, frame: func(frame int, r, g, b byte, brightnessPercent, speedPercent, numLEDs int) ([]hardware.Color, time.Duration) {
		return RainbowFrame(frame, brightnessPercent, numLEDs, false), RainbowDelay(speedPercent)
	}},
	{name: StyleRainbowReverse, frame: func(frame int, r, g, b byte, brightnessPercent, speedPercent, numLEDs int) ([]hardware.Color, time.Duration) {
		return RainbowFrame(frame, brightnessPercent, numLEDs, true), RainbowDelay(speedPercent)
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

func speedToDelay(speedPercent int, maxDelay, minDelay time.Duration) time.Duration {
	span := maxDelay - minDelay
	return maxDelay - span*time.Duration(speedPercent)/100
}

func scaleColor(r, g, b byte, percent int) hardware.Color {
	return hardware.Color{R: scale(r, percent), G: scale(g, percent), B: scale(b, percent)}
}

func mirrorIndex(i, numLEDs int, reverse bool) int {
	if reverse {
		return numLEDs - 1 - i
	}
	return i
}

// 200 matches original pironman 5 breathing cycle.
const breathingSteps = 200

func BreathingFrame(frame int, r, g, b byte, brightnessPercent, numLEDs int) []hardware.Color {
	pos := frame % breathingSteps
	half := breathingSteps / 2

	levelPercent := pos * 99 / half
	if pos >= half {
		levelPercent = (breathingSteps - pos) * 99 / half
	}

	scaledPercent := brightnessPercent * levelPercent / 100
	color := scaleColor(r, g, b, scaledPercent)

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
	breathingDelayMax = 100 * time.Millisecond
	breathingDelayMin = 1 * time.Millisecond
)

func BreathingDelay(speedPercent int) time.Duration {
	return speedToDelay(speedPercent, breathingDelayMax, breathingDelayMin)
}

// FlowFrame lights exactly one LED at a time, advancing one physical index
// per frame; reverse walks the indices in the opposite order.
func FlowFrame(frame int, r, g, b byte, brightnessPercent, numLEDs int, reverse bool) []hardware.Color {
	pos := mirrorIndex(frame%numLEDs, numLEDs, reverse)

	pixels := make([]hardware.Color, numLEDs)
	pixels[pos] = scaleColor(r, g, b, brightnessPercent)
	return pixels
}

const (
	flowDelayMax = 500 * time.Millisecond
	flowDelayMin = 100 * time.Millisecond
)

func FlowDelay(speedPercent int) time.Duration {
	return speedToDelay(speedPercent, flowDelayMax, flowDelayMin)
}

func HSLToRGB(h float64, s, l int) (r, g, b byte) {
	hh := math.Mod(h, 360)
	if hh < 0 {
		hh += 360
	}
	sf := float64(s) / 100
	lf := float64(l) / 100

	c := (1 - math.Abs(2*lf-1)) * sf
	x := c * (1 - math.Abs(math.Mod(hh/60, 2)-1))
	m := lf - c/2

	var rp, gp, bp float64
	switch {
	case hh < 60:
		rp, gp, bp = c, x, 0
	case hh < 120:
		rp, gp, bp = x, c, 0
	case hh < 180:
		rp, gp, bp = 0, c, x
	case hh < 240:
		rp, gp, bp = 0, x, c
	case hh < 300:
		rp, gp, bp = x, 0, c
	default:
		rp, gp, bp = c, 0, x
	}

	return byte((rp + m) * 255), byte((gp + m) * 255), byte((bp + m) * 255)
}

const rainbowCycleDegrees = 360

func RainbowFrame(frame int, brightnessPercent, numLEDs int, reverse bool) []hardware.Color {
	phase := frame % rainbowCycleDegrees

	pixels := make([]hardware.Color, numLEDs)
	for i := range pixels {
		idx := mirrorIndex(i, numLEDs, reverse)
		hue := float64((idx*rainbowCycleDegrees/numLEDs + phase) % rainbowCycleDegrees)
		r, g, b := HSLToRGB(hue, 100, 50)
		pixels[i] = scaleColor(r, g, b, brightnessPercent)
	}
	return pixels
}

const (
	rainbowDelayMax = 100 * time.Millisecond
	rainbowDelayMin = 5 * time.Millisecond
)

func RainbowDelay(speedPercent int) time.Duration {
	return speedToDelay(speedPercent, rainbowDelayMax, rainbowDelayMin)
}

func animationFrame(style string, frame int, r, g, b byte, brightnessPercent, speedPercent, numLEDs int) ([]hardware.Color, time.Duration, error) {
	d, ok := lookupStyle(style)
	if !ok || d.frame == nil {
		return nil, 0, fmt.Errorf("rgb: unknown animated style %q", style)
	}
	pixels, delay := d.frame(frame, r, g, b, brightnessPercent, speedPercent, numLEDs)
	return pixels, delay, nil
}
