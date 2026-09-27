package rgb_test

import (
	"errors"
	"testing"

	"github.com/dnitros/pironman/internal/rgb"
)

var errBoom = errors.New("boom")

type fakeStrip struct {
	onCalls, offCalls int
	onErr, offErr     error
}

func (f *fakeStrip) On() error {
	f.onCalls++
	return f.onErr
}

func (f *fakeStrip) Off() error {
	f.offCalls++
	return f.offErr
}

func TestNewStoreAppliesEnabledInitialStateOnce(t *testing.T) {
	strip := &fakeStrip{}

	if _, err := rgb.NewStore(strip, rgb.State{Enabled: true}); err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if strip.onCalls != 1 || strip.offCalls != 0 {
		t.Fatalf("expected exactly one On() call, got on=%d off=%d", strip.onCalls, strip.offCalls)
	}
}

func TestNewStoreAppliesDisabledInitialStateOnce(t *testing.T) {
	strip := &fakeStrip{}

	if _, err := rgb.NewStore(strip, rgb.State{Enabled: false}); err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if strip.offCalls != 1 || strip.onCalls != 0 {
		t.Fatalf("expected exactly one Off() call, got on=%d off=%d", strip.onCalls, strip.offCalls)
	}
}

func TestNewStorePropagatesApplyError(t *testing.T) {
	strip := &fakeStrip{onErr: errBoom}

	if _, err := rgb.NewStore(strip, rgb.State{Enabled: true}); err == nil {
		t.Fatalf("expected an error when the initial apply fails")
	}
}

func TestStoreOnTurnsStripOnAndUpdatesState(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	state, err := store.On()
	if err != nil {
		t.Fatalf("On: %v", err)
	}
	if !state.Enabled || !store.Enabled() {
		t.Fatalf("expected state to be enabled after On()")
	}
	if strip.onCalls != 1 {
		t.Fatalf("expected On() to call strip.On() once, got %d", strip.onCalls)
	}
}

func TestStoreOffTurnsStripOffAndUpdatesState(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	state, err := store.Off()
	if err != nil {
		t.Fatalf("Off: %v", err)
	}
	if state.Enabled || store.Enabled() {
		t.Fatalf("expected state to be disabled after Off()")
	}
	if strip.offCalls != 1 {
		t.Fatalf("expected strip.Off() to be called once, got %d", strip.offCalls)
	}
}

func TestStoreOnPropagatesHardwareErrorWithoutChangingState(t *testing.T) {
	strip := &fakeStrip{onErr: errBoom}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	if _, err := store.On(); err == nil {
		t.Fatalf("expected On() to propagate the hardware error")
	}
	if store.Enabled() {
		t.Fatalf("expected state to remain disabled after a failed On()")
	}
}

func TestStorePersistsAcrossSimulatedRestart(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if _, err := store.Off(); err != nil {
		t.Fatalf("Off: %v", err)
	}

	restartedStrip := &fakeStrip{}
	restarted, err := rgb.NewStore(restartedStrip, rgb.State{Enabled: store.Enabled()})
	if err != nil {
		t.Fatalf("NewStore (restart): %v", err)
	}

	if restarted.Enabled() {
		t.Fatalf("expected the disabled state to survive the simulated restart")
	}
	if restartedStrip.offCalls != 1 || restartedStrip.onCalls != 0 {
		t.Fatalf("expected the restarted store to reapply Off() once, got on=%d off=%d", restartedStrip.onCalls, restartedStrip.offCalls)
	}
}
