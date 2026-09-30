package powerbutton_test

import (
	"sync"
	"testing"
	"time"

	"github.com/dnitros/pironman/internal/clock"
	"github.com/dnitros/pironman/internal/powerbutton"
)

type fakeClock struct {
	t time.Time
}

func (f *fakeClock) Now() time.Time { return f.t }

func (f *fakeClock) Advance(d time.Duration) { f.t = f.t.Add(d) }

func newFakeClock() *fakeClock { return &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)} }

func TestClassifierConfirmsClickAfterDebounceWindowElapses(t *testing.T) {
	clk := newFakeClock()
	c := powerbutton.NewClassifier(clk)

	if got := c.PressDown(); got != powerbutton.EventNone {
		t.Fatalf("PressDown: got %v, want EventNone", got)
	}
	clk.Advance(50 * time.Millisecond)
	if got := c.PressUp(); got != powerbutton.EventNone {
		t.Fatalf("PressUp: got %v, want EventNone (click is provisional)", got)
	}

	// The debounce window is measured from the press-down, not the release
	// (50ms elapsed already), so only advance up to just under the total.
	clk.Advance(powerbutton.DebounceWindow - 50*time.Millisecond - time.Millisecond)
	if got := c.Tick(); got != powerbutton.EventNone {
		t.Fatalf("Tick before window elapses: got %v, want EventNone", got)
	}

	clk.Advance(2 * time.Millisecond)
	if got := c.Tick(); got != powerbutton.EventClick {
		t.Fatalf("Tick after window elapses: got %v, want EventClick", got)
	}
}

func TestClassifierSupersedesPendingClickWithDoubleClick(t *testing.T) {
	clk := newFakeClock()
	c := powerbutton.NewClassifier(clk)

	c.PressDown()
	clk.Advance(30 * time.Millisecond)
	c.PressUp()

	clk.Advance(100 * time.Millisecond)
	if got := c.PressDown(); got != powerbutton.EventDoubleClick {
		t.Fatalf("second PressDown within window: got %v, want EventDoubleClick", got)
	}

	clk.Advance(200 * time.Millisecond)
	if got := c.Tick(); got != powerbutton.EventNone {
		t.Fatalf("Tick after double-click resolved: got %v, want EventNone (no leftover click)", got)
	}

	if got := c.PressUp(); got != powerbutton.EventNone {
		t.Fatalf("release of the second press: got %v, want EventNone", got)
	}
}

func TestClassifierFlushesPendingClickWhenSecondPressArrivesAfterWindowExpires(t *testing.T) {
	clk := newFakeClock()
	c := powerbutton.NewClassifier(clk)

	c.PressDown()
	clk.Advance(30 * time.Millisecond)
	c.PressUp()

	clk.Advance(powerbutton.DebounceWindow + 10*time.Millisecond)
	if got := c.PressDown(); got != powerbutton.EventClick {
		t.Fatalf("PressDown after window expired: got %v, want EventClick (flushed, not dropped)", got)
	}
}

func TestClassifierDetectsLongPressThresholdCrossing(t *testing.T) {
	clk := newFakeClock()
	c := powerbutton.NewClassifier(clk)

	c.PressDown()
	clk.Advance(powerbutton.LongPressThreshold)
	if got := c.Tick(); got != powerbutton.EventLongPress {
		t.Fatalf("Tick at threshold: got %v, want EventLongPress", got)
	}
}

func TestClassifierRefiresLongPressIdempotentlyWhileHeld(t *testing.T) {
	clk := newFakeClock()
	c := powerbutton.NewClassifier(clk)

	c.PressDown()
	clk.Advance(powerbutton.LongPressThreshold)
	for i := 0; i < 3; i++ {
		clk.Advance(time.Second)
		if got := c.Tick(); got != powerbutton.EventLongPress {
			t.Fatalf("Tick #%d while held: got %v, want EventLongPress", i, got)
		}
	}
}

func TestClassifierLongPressReleasedHasNoUpperBound(t *testing.T) {
	clk := newFakeClock()
	c := powerbutton.NewClassifier(clk)

	c.PressDown()
	clk.Advance(powerbutton.LongPressThreshold + time.Minute)
	if got := c.PressUp(); got != powerbutton.EventLongPressReleased {
		t.Fatalf("PressUp after a very long hold: got %v, want EventLongPressReleased", got)
	}
}

// TestClassifierIsSafeForConcurrentPressAndTick guards against the exact
// shape the daemon drives the classifier in: PressDown/PressUp from the
// watcher goroutine and Tick from the ticker goroutine, at the same time.
// It uses the real clock, since it's only checking for data races (run with
// -race), not classification timing (covered by the tests above).
func TestClassifierIsSafeForConcurrentPressAndTick(t *testing.T) {
	c := powerbutton.NewClassifier(clock.RealClock{})
	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				c.PressDown()
				c.PressUp()
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				c.Tick()
			}
		}
	}()

	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestClassifierShortPressReleasedBeforeThresholdIsNotLongPress(t *testing.T) {
	clk := newFakeClock()
	c := powerbutton.NewClassifier(clk)

	c.PressDown()
	clk.Advance(powerbutton.LongPressThreshold - time.Millisecond)
	if got := c.PressUp(); got != powerbutton.EventNone {
		t.Fatalf("PressUp just under the threshold: got %v, want EventNone (provisional click)", got)
	}
}
