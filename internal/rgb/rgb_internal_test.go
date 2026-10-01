package rgb

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dnitros/pironman/internal/hardware"
)

var errFakeStripOn = errors.New("fake strip on failed")

// fakeStrip is a minimal, mutex-protected WS2812Strip fake for exercising
// the animation goroutine, which writes frames concurrently with the test.
type fakeStrip struct {
	mu              sync.Mutex
	onCalls         int
	offCalls        int
	setColorCalls   int
	writeFrameCalls int
	onErr           error
}

func (f *fakeStrip) On() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onCalls++
	return f.onErr
}
func (f *fakeStrip) Off() error { f.mu.Lock(); defer f.mu.Unlock(); f.offCalls++; return nil }
func (f *fakeStrip) SetColor(r, g, b byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setColorCalls++
}
func (f *fakeStrip) WriteFrame(pixels []hardware.Color) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeFrameCalls++
	return nil
}
func (f *fakeStrip) counts() (on, off, setColor, writeFrame int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.onCalls, f.offCalls, f.setColorCalls, f.writeFrameCalls
}

// withNoOpSleep overrides the package's animation-loop sleep so tests never
// wait on real wall-clock time, and restores it on cleanup.
func withNoOpSleep(t *testing.T) {
	t.Helper()
	orig := sleep
	sleep = func(time.Duration) {}
	t.Cleanup(func() { sleep = orig })
}

// withSteppedSleep overrides sleep with a fake that reports each completed
// frame on frameDone and then blocks on step, letting the test pace the
// animation loop one frame at a time.
func withSteppedSleep(t *testing.T) (frameDone <-chan struct{}, step chan<- struct{}) {
	t.Helper()
	orig := sleep
	// Buffered so the loop's own "frame written" report never blocks once the
	// test stops draining it (e.g. while racing a concurrent cancellation).
	done := make(chan struct{}, 8)
	gate := make(chan struct{})
	sleep = func(time.Duration) {
		done <- struct{}{}
		<-gate
	}
	t.Cleanup(func() { sleep = orig })
	return done, gate
}

func TestNewStoreStartsAnimationForEnabledAnimatedStyle(t *testing.T) {
	withNoOpSleep(t)
	strip := &fakeStrip{}
	store, err := NewStore(strip, State{Enabled: true, Style: StyleBreathing, Color: "#ff0000", Brightness: 100, Speed: 50})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.mu.Lock()
	running := store.animCancel != nil
	store.mu.Unlock()
	if !running {
		t.Fatalf("expected the animation loop to be running for an enabled, animated initial state")
	}

	store.Off()
}

func TestNewStoreDoesNotStartAnimationForSolidOrDisabled(t *testing.T) {
	withNoOpSleep(t)
	for _, initial := range []State{
		{Enabled: true, Style: StyleSolid},
		{Enabled: false, Style: StyleBreathing},
	} {
		strip := &fakeStrip{}
		store, err := NewStore(strip, initial)
		if err != nil {
			t.Fatalf("NewStore(%+v): %v", initial, err)
		}

		store.mu.Lock()
		running := store.animCancel != nil
		store.mu.Unlock()
		if running {
			t.Fatalf("NewStore(%+v): expected no animation loop running", initial)
		}
		if _, _, _, writeFrame := strip.counts(); writeFrame != 0 {
			t.Fatalf("NewStore(%+v): expected no frames written, got %d", initial, writeFrame)
		}
	}
}

func TestAnimationLoopWritesFramesUntilStopped(t *testing.T) {
	frameDone, step := withSteppedSleep(t)
	strip := &fakeStrip{}
	store, err := NewStore(strip, State{Enabled: true, Style: StyleBreathing, Color: "#ff0000", Brightness: 100, Speed: 50})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	<-frameDone // frame 0 written
	if _, _, _, writeFrame := strip.counts(); writeFrame != 1 {
		t.Fatalf("expected 1 frame written, got %d", writeFrame)
	}

	step <- struct{}{} // allow frame 1
	<-frameDone
	if _, _, _, writeFrame := strip.counts(); writeFrame != 2 {
		t.Fatalf("expected 2 frames written, got %d", writeFrame)
	}

	offDone := make(chan struct{})
	go func() {
		store.Off()
		close(offDone)
	}()
	// Cancellation races the sleep gate: keep pumping both channels until the
	// loop notices ctx.Done() and Off() returns, however many frames that takes.
drain:
	for {
		select {
		case <-offDone:
			break drain
		case step <- struct{}{}:
		case <-frameDone:
		}
	}

	if _, off, _, _ := strip.counts(); off != 1 {
		t.Fatalf("expected strip.Off() to be called once, got %d", off)
	}
	store.mu.Lock()
	running := store.animCancel != nil
	store.mu.Unlock()
	if running {
		t.Fatalf("expected the animation loop to have stopped")
	}
}

func TestOnResumesConfiguredAnimatedStyleWithoutOneShotApply(t *testing.T) {
	withNoOpSleep(t)
	strip := &fakeStrip{}
	store, err := NewStore(strip, State{Enabled: false, Style: StyleBreathing, Color: "#00ff00", Brightness: 100, Speed: 50})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if _, err := store.On(); err != nil {
		t.Fatalf("On: %v", err)
	}

	store.mu.Lock()
	running := store.animCancel != nil
	store.mu.Unlock()
	if !running {
		t.Fatalf("expected On() to start the animation loop for an animated style")
	}
	if on, _, _, _ := strip.counts(); on != 0 {
		t.Fatalf("expected strip.On() to never be called for an animated style, got %d", on)
	}

	store.Off()
}

func TestSetStyleSwitchingToSolidStopsLoopAndAppliesOnce(t *testing.T) {
	withNoOpSleep(t)
	strip := &fakeStrip{}
	store, err := NewStore(strip, State{Enabled: true, Style: StyleBreathing, Color: "#0000ff", Brightness: 100, Speed: 50})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if _, err := store.SetStyle(StyleSolid, nil); err != nil {
		t.Fatalf("SetStyle: %v", err)
	}

	store.mu.Lock()
	running := store.animCancel != nil
	store.mu.Unlock()
	if running {
		t.Fatalf("expected switching to solid to stop the animation loop")
	}
	if on, _, setColor, _ := strip.counts(); on != 1 || setColor != 1 {
		t.Fatalf("expected one one-shot apply (On=1, SetColor=1), got On=%d SetColor=%d", on, setColor)
	}
}

func TestSetStyleSwitchingToAnimatedStartsLoop(t *testing.T) {
	withNoOpSleep(t)
	strip := &fakeStrip{}
	store, err := NewStore(strip, State{Enabled: true, Style: StyleSolid, Color: "#0000ff", Brightness: 100})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if _, err := store.SetStyle(StyleBreathing, nil); err != nil {
		t.Fatalf("SetStyle: %v", err)
	}

	store.mu.Lock()
	running := store.animCancel != nil
	store.mu.Unlock()
	if !running {
		t.Fatalf("expected switching to breathing to start the animation loop")
	}

	store.Off()
}

func TestSetStyleSpeedOnlyChangeDoesNotRestartLoop(t *testing.T) {
	withNoOpSleep(t)
	strip := &fakeStrip{}
	store, err := NewStore(strip, State{Enabled: true, Style: StyleBreathing, Color: "#0000ff", Brightness: 100, Speed: 10})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	store.mu.Lock()
	before := store.animDone
	store.mu.Unlock()

	newSpeed := 90
	state, err := store.SetStyle(StyleBreathing, &newSpeed)
	if err != nil {
		t.Fatalf("SetStyle: %v", err)
	}
	if state.Speed != 90 {
		t.Fatalf("expected speed to update to 90, got %d", state.Speed)
	}

	store.mu.Lock()
	after := store.animDone
	store.mu.Unlock()
	if before != after {
		t.Fatalf("expected the animation loop to keep running (same goroutine) across a speed-only change")
	}

	store.Off()
}

func TestSetStyleSwitchingToSolidRevertsOnFailedApply(t *testing.T) {
	withNoOpSleep(t)
	strip := &fakeStrip{}
	store, err := NewStore(strip, State{Enabled: true, Style: StyleBreathing, Color: "#0000ff", Brightness: 100, Speed: 50})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	strip.mu.Lock()
	strip.onErr = errFakeStripOn
	strip.mu.Unlock()

	state, err := store.SetStyle(StyleSolid, nil)
	if err == nil {
		t.Fatalf("expected SetStyle to propagate the hardware error")
	}
	if state.Style != StyleBreathing {
		t.Fatalf("expected style to be rolled back to breathing on a failed apply, got %q", state.Style)
	}
	if store.State().Style != StyleBreathing {
		t.Fatalf("expected the store's state to stay breathing after a failed switch to solid, got %q", store.State().Style)
	}

	store.mu.Lock()
	running := store.animCancel != nil
	store.mu.Unlock()
	if !running {
		t.Fatalf("expected the animation loop to resume after a rolled-back style switch")
	}

	store.Off()
}

func TestSetStyleRejectsInvalidNameWithoutMutatingState(t *testing.T) {
	strip := &fakeStrip{}
	store, err := NewStore(strip, State{Enabled: false, Style: StyleSolid})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if _, err := store.SetStyle("rainbow", nil); err == nil {
		t.Fatalf("expected an error for an invalid style name")
	}
	if store.State().Style != StyleSolid {
		t.Fatalf("expected style to remain unchanged after a rejected SetStyle")
	}
}

func TestSetStyleRejectsInvalidSpeedWithoutMutatingState(t *testing.T) {
	strip := &fakeStrip{}
	store, err := NewStore(strip, State{Enabled: false, Style: StyleSolid, Speed: 50})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	badSpeed := 101
	if _, err := store.SetStyle(StyleBreathing, &badSpeed); err == nil {
		t.Fatalf("expected an error for an out-of-range speed")
	}
	if store.State().Style != StyleSolid || store.State().Speed != 50 {
		t.Fatalf("expected state to remain unchanged after a rejected SetStyle, got %+v", store.State())
	}
}
