package rgb_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dnitros/pironman/internal/rgb"
)

func TestValidateStyleAcceptsKnownStyles(t *testing.T) {
	for _, name := range []string{"solid", "breathing"} {
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
