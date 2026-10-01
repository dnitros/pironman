package rgb_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/rgb"
)

func TestValidateStyleAcceptsKnownStyles(t *testing.T) {
	for _, name := range []string{"solid", "breathing", "flow", "flow_reverse"} {
		if err := rgb.ValidateStyle(name); err != nil {
			t.Fatalf("ValidateStyle(%q): unexpected error: %v", name, err)
		}
	}
}

func TestValidateStyleRejectsUnknownName(t *testing.T) {
	err := rgb.ValidateStyle("rainbow")
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
