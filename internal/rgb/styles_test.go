package rgb_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/rgb"
)

func TestValidateStyleAcceptsKnownStyles(t *testing.T) {
	for _, name := range []string{"solid", "breathing", "flow", "flow_reverse", "rainbow", "rainbow_reverse", "hue_cycle"} {
		if err := rgb.ValidateStyle(name); err != nil {
			t.Fatalf("ValidateStyle(%q): unexpected error: %v", name, err)
		}
	}
}

func TestValidateStyleRejectsUnknownName(t *testing.T) {
	err := rgb.ValidateStyle("disco")
	if err == nil {
		t.Fatalf("expected an error for an unimplemented style name")
	}
	if !strings.Contains(err.Error(), "solid") || !strings.Contains(err.Error(), "breathing") {
		t.Fatalf("expected the error to list valid options, got: %v", err)
	}
}

func TestValidateSpeedRejectsOutOfRange(t *testing.T) {
	for _, pct := range []int{-1, 101, 1000} {
		if err := rgb.ValidateSpeed(pct); err == nil {
			t.Fatalf("ValidateSpeed(%d): expected an error", pct)
		}
	}
}

func TestValidateSpeedAcceptsBoundaries(t *testing.T) {
	for _, pct := range []int{0, 50, 100} {
		if err := rgb.ValidateSpeed(pct); err != nil {
			t.Fatalf("ValidateSpeed(%d): unexpected error: %v", pct, err)
		}
	}
}

func TestBreathingFrameAtZeroIsFullyDark(t *testing.T) {
	frame := rgb.BreathingFrame(0, 0xff, 0xff, 0xff, 100, 3)
	if len(frame) != 3 {
		t.Fatalf("expected 3 pixels, got %d", len(frame))
	}
	for i, p := range frame {
		if p.R != 0 || p.G != 0 || p.B != 0 {
			t.Fatalf("pixel %d: expected fully dark at frame 0, got %+v", i, p)
		}
	}
}

func TestBreathingFrameAtMidCycleIsNearlyFullBrightness(t *testing.T) {
	frame := rgb.BreathingFrame(100, 0xff, 0, 0, 100, 1)
	want := byte(0xff * 99 / 100)
	if frame[0].R != want {
		t.Fatalf("expected R near peak brightness (%d), got %d", want, frame[0].R)
	}
}

func TestBreathingFrameWrapsAtCycleLength(t *testing.T) {
	a := rgb.BreathingFrame(5, 10, 20, 30, 100, 1)
	b := rgb.BreathingFrame(205, 10, 20, 30, 100, 1)
	if a[0] != b[0] {
		t.Fatalf("expected the 200-step cycle to repeat: frame 5 = %+v, frame 205 = %+v", a[0], b[0])
	}
}

func TestBreathingFrameScalesByConfiguredBrightness(t *testing.T) {
	full := rgb.BreathingFrame(100, 0xff, 0, 0, 100, 1)
	half := rgb.BreathingFrame(100, 0xff, 0, 0, 50, 1)
	if half[0].R >= full[0].R {
		t.Fatalf("expected halving the configured brightness to dim the frame: full=%d half=%d", full[0].R, half[0].R)
	}
}

func TestBreathingDelayAtSpeedZeroIsSlowest(t *testing.T) {
	got := rgb.BreathingDelay(0)
	if got != 100*time.Millisecond {
		t.Fatalf("BreathingDelay(0) = %v, want 100ms", got)
	}
}

func TestBreathingDelayAtSpeedHundredIsFastest(t *testing.T) {
	got := rgb.BreathingDelay(100)
	if got != 1*time.Millisecond {
		t.Fatalf("BreathingDelay(100) = %v, want 1ms", got)
	}
}

func TestBreathingDelayIsMonotonicallyDecreasing(t *testing.T) {
	prev := rgb.BreathingDelay(0)
	for speed := 10; speed <= 100; speed += 10 {
		got := rgb.BreathingDelay(speed)
		if got > prev {
			t.Fatalf("BreathingDelay(%d) = %v, expected <= previous %v", speed, got, prev)
		}
		prev = got
	}
}

func litIndex(t *testing.T, pixels []hardware.Color) int {
	t.Helper()
	lit := -1
	for i, p := range pixels {
		if p != (hardware.Color{}) {
			if lit != -1 {
				t.Fatalf("expected exactly one lit pixel, found a second at index %d (first at %d)", i, lit)
			}
			lit = i
		}
	}
	if lit == -1 {
		t.Fatalf("expected exactly one lit pixel, found none")
	}
	return lit
}

func TestFlowFrameLightsExactlyOnePixelAtStart(t *testing.T) {
	pixels := rgb.FlowFrame(0, 0xff, 0, 0, 100, 4, false)
	if len(pixels) != 4 {
		t.Fatalf("expected 4 pixels, got %d", len(pixels))
	}
	if got := litIndex(t, pixels); got != 0 {
		t.Fatalf("FlowFrame(0, ...): expected lit index 0, got %d", got)
	}
}

func TestFlowFrameAdvancesOnePositionPerFrame(t *testing.T) {
	for frame, want := range map[int]int{1: 1, 2: 2, 3: 3} {
		got := litIndex(t, rgb.FlowFrame(frame, 0xff, 0, 0, 100, 4, false))
		if got != want {
			t.Fatalf("FlowFrame(%d, ...): expected lit index %d, got %d", frame, want, got)
		}
	}
}

func TestFlowFrameWrapsAtStripLength(t *testing.T) {
	got := litIndex(t, rgb.FlowFrame(4, 0xff, 0, 0, 100, 4, false))
	if got != 0 {
		t.Fatalf("FlowFrame(4, ...): expected to wrap to index 0, got %d", got)
	}
}

func TestFlowFrameReverseStartsAtLastIndex(t *testing.T) {
	got := litIndex(t, rgb.FlowFrame(0, 0xff, 0, 0, 100, 4, true))
	if got != 3 {
		t.Fatalf("FlowFrame(0, ..., reverse): expected lit index 3, got %d", got)
	}
}

func TestFlowFrameReverseAdvancesInOppositeDirection(t *testing.T) {
	for frame, want := range map[int]int{1: 2, 2: 1, 3: 0, 4: 3} {
		got := litIndex(t, rgb.FlowFrame(frame, 0xff, 0, 0, 100, 4, true))
		if got != want {
			t.Fatalf("FlowFrame(%d, ..., reverse): expected lit index %d, got %d", frame, want, got)
		}
	}
}

func TestFlowFrameScalesLitPixelByBrightness(t *testing.T) {
	full := rgb.FlowFrame(0, 0xff, 0, 0, 100, 4, false)
	half := rgb.FlowFrame(0, 0xff, 0, 0, 50, 4, false)
	if half[0].R >= full[0].R {
		t.Fatalf("expected halving brightness to dim the lit pixel: full=%d half=%d", full[0].R, half[0].R)
	}
}

func TestFlowDelayAtSpeedZeroIsSlowest(t *testing.T) {
	got := rgb.FlowDelay(0)
	if got != 500*time.Millisecond {
		t.Fatalf("FlowDelay(0) = %v, want 500ms", got)
	}
}

func TestFlowDelayAtSpeedHundredIsFastest(t *testing.T) {
	got := rgb.FlowDelay(100)
	if got != 100*time.Millisecond {
		t.Fatalf("FlowDelay(100) = %v, want 100ms", got)
	}
}

func TestFlowDelayIsMonotonicallyDecreasing(t *testing.T) {
	prev := rgb.FlowDelay(0)
	for speed := 10; speed <= 100; speed += 10 {
		got := rgb.FlowDelay(speed)
		if got > prev {
			t.Fatalf("FlowDelay(%d) = %v, expected <= previous %v", speed, got, prev)
		}
		prev = got
	}
}

func TestHSLToRGBPrimaryHuesAtFullSaturationHalfLightness(t *testing.T) {
	cases := []struct {
		hue     float64
		r, g, b byte
	}{
		{0, 255, 0, 0},
		{60, 255, 255, 0},
		{120, 0, 255, 0},
		{180, 0, 255, 255},
		{240, 0, 0, 255},
		{300, 255, 0, 255},
	}
	for _, c := range cases {
		r, g, b := rgb.HSLToRGB(c.hue, 100, 50)
		if r != c.r || g != c.g || b != c.b {
			t.Fatalf("HSLToRGB(%v, 100, 50) = (%d,%d,%d), want (%d,%d,%d)", c.hue, r, g, b, c.r, c.g, c.b)
		}
	}
}

func TestHSLToRGBZeroLightnessIsBlackRegardlessOfHue(t *testing.T) {
	for _, hue := range []float64{0, 90, 200, 359} {
		r, g, b := rgb.HSLToRGB(hue, 100, 0)
		if r != 0 || g != 0 || b != 0 {
			t.Fatalf("HSLToRGB(%v, 100, 0) = (%d,%d,%d), want black", hue, r, g, b)
		}
	}
}

func TestHSLToRGBFullLightnessIsWhiteRegardlessOfHue(t *testing.T) {
	for _, hue := range []float64{0, 90, 200, 359} {
		r, g, b := rgb.HSLToRGB(hue, 100, 100)
		if r != 255 || g != 255 || b != 255 {
			t.Fatalf("HSLToRGB(%v, 100, 100) = (%d,%d,%d), want white", hue, r, g, b)
		}
	}
}

func TestHSLToRGBZeroSaturationIsGreyRegardlessOfHue(t *testing.T) {
	for _, hue := range []float64{0, 90, 200, 359} {
		r, g, b := rgb.HSLToRGB(hue, 0, 50)
		if r != g || g != b {
			t.Fatalf("HSLToRGB(%v, 0, 50) = (%d,%d,%d), want r == g == b", hue, r, g, b)
		}
	}
}

func TestHSLToRGBNormalizesHueOutsideZeroTo360(t *testing.T) {
	r1, g1, b1 := rgb.HSLToRGB(0, 100, 50)
	r2, g2, b2 := rgb.HSLToRGB(360, 100, 50)
	if r1 != r2 || g1 != g2 || b1 != b2 {
		t.Fatalf("HSLToRGB(360, ...) = (%d,%d,%d), want same as HSLToRGB(0, ...) = (%d,%d,%d)", r2, g2, b2, r1, g1, b1)
	}
}

func TestRainbowFrameAssignsEvenlySpacedHuesAtFrameZero(t *testing.T) {
	pixels := rgb.RainbowFrame(0, 100, 6, false)
	if len(pixels) != 6 {
		t.Fatalf("expected 6 pixels, got %d", len(pixels))
	}
	want := []hardware.Color{
		{R: 255, G: 0, B: 0},
		{R: 255, G: 255, B: 0},
		{R: 0, G: 255, B: 0},
		{R: 0, G: 255, B: 255},
		{R: 0, G: 0, B: 255},
		{R: 255, G: 0, B: 255},
	}
	for i := range want {
		if pixels[i] != want[i] {
			t.Fatalf("pixel %d = %+v, want %+v", i, pixels[i], want[i])
		}
	}
}

func TestRainbowFrameRotatesByOneDegreePerFrame(t *testing.T) {
	atFrame0 := rgb.RainbowFrame(0, 100, 6, false)
	atFrame60 := rgb.RainbowFrame(60, 100, 6, false)
	if atFrame60[0] != atFrame0[1] {
		t.Fatalf("RainbowFrame(60, ...)[0] = %+v, want it to match RainbowFrame(0, ...)[1] = %+v (one full 60-degree rotation)", atFrame60[0], atFrame0[1])
	}
}

func TestRainbowFrameWrapsAtCycleLength(t *testing.T) {
	atFrame0 := rgb.RainbowFrame(0, 100, 6, false)
	atFrame360 := rgb.RainbowFrame(360, 100, 6, false)
	for i := range atFrame0 {
		if atFrame0[i] != atFrame360[i] {
			t.Fatalf("pixel %d: expected the 360-degree cycle to repeat: frame 0 = %+v, frame 360 = %+v", i, atFrame0[i], atFrame360[i])
		}
	}
}

func TestRainbowReverseFrameAssignsPatternInReverseIndexOrder(t *testing.T) {
	forward := rgb.RainbowFrame(0, 100, 6, false)
	reverse := rgb.RainbowFrame(0, 100, 6, true)
	for i := range forward {
		if reverse[i] != forward[len(forward)-1-i] {
			t.Fatalf("reverse pixel %d = %+v, want forward pixel %d = %+v", i, reverse[i], len(forward)-1-i, forward[len(forward)-1-i])
		}
	}
}

func TestRainbowFrameScalesByBrightness(t *testing.T) {
	full := rgb.RainbowFrame(0, 100, 6, false)
	half := rgb.RainbowFrame(0, 50, 6, false)
	if half[0].R >= full[0].R {
		t.Fatalf("expected halving brightness to dim the frame: full=%d half=%d", full[0].R, half[0].R)
	}
}

func TestRainbowDelayAtSpeedZeroIsSlowest(t *testing.T) {
	got := rgb.RainbowDelay(0)
	if got != 100*time.Millisecond {
		t.Fatalf("RainbowDelay(0) = %v, want 100ms", got)
	}
}

func TestRainbowDelayAtSpeedHundredIsFastest(t *testing.T) {
	got := rgb.RainbowDelay(100)
	if got != 5*time.Millisecond {
		t.Fatalf("RainbowDelay(100) = %v, want 5ms", got)
	}
}

func TestRainbowDelayIsMonotonicallyDecreasing(t *testing.T) {
	prev := rgb.RainbowDelay(0)
	for speed := 10; speed <= 100; speed += 10 {
		got := rgb.RainbowDelay(speed)
		if got > prev {
			t.Fatalf("RainbowDelay(%d) = %v, expected <= previous %v", speed, got, prev)
		}
		prev = got
	}
}

func TestHueCycleFrameUsesSameHueForAllLEDs(t *testing.T) {
	pixels := rgb.HueCycleFrame(0, 100, 4)
	if len(pixels) != 4 {
		t.Fatalf("expected 4 pixels, got %d", len(pixels))
	}
	want := hardware.Color{R: 255, G: 0, B: 0}
	for i, p := range pixels {
		if p != want {
			t.Fatalf("pixel %d = %+v, want %+v (uniform hue across the strip)", i, p, want)
		}
	}
}

func TestHueCycleFrameAdvancesByOneDegreePerFrame(t *testing.T) {
	pixels := rgb.HueCycleFrame(60, 100, 3)
	want := hardware.Color{R: 255, G: 255, B: 0}
	for i, p := range pixels {
		if p != want {
			t.Fatalf("pixel %d = %+v, want %+v (60 degrees in = yellow)", i, p, want)
		}
	}
}

func TestHueCycleFrameWrapsAtCycleLength(t *testing.T) {
	atFrame0 := rgb.HueCycleFrame(0, 100, 3)
	atFrame360 := rgb.HueCycleFrame(360, 100, 3)
	for i := range atFrame0 {
		if atFrame0[i] != atFrame360[i] {
			t.Fatalf("pixel %d: expected the 360-degree cycle to repeat: frame 0 = %+v, frame 360 = %+v", i, atFrame0[i], atFrame360[i])
		}
	}
}

func TestHueCycleFrameScalesByBrightness(t *testing.T) {
	full := rgb.HueCycleFrame(0, 100, 3)
	half := rgb.HueCycleFrame(0, 50, 3)
	if half[0].R >= full[0].R {
		t.Fatalf("expected halving brightness to dim the frame: full=%d half=%d", full[0].R, half[0].R)
	}
}
