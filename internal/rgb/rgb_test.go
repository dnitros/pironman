package rgb_test

import (
	"errors"
	"testing"

	"github.com/dnitros/pironman/internal/rgb"
)

var errBoom = errors.New("boom")

type fakeStrip struct {
	onCalls, offCalls, setColorCalls int
	onErr, offErr                    error
	lastR, lastG, lastB              byte
}

func (f *fakeStrip) On() error {
	f.onCalls++
	return f.onErr
}

func (f *fakeStrip) Off() error {
	f.offCalls++
	return f.offErr
}

func (f *fakeStrip) SetColor(r, g, b byte) {
	f.setColorCalls++
	f.lastR, f.lastG, f.lastB = r, g, b
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

func TestStoreSetColorAppliesImmediatelyWhenEnabled(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	state, err := store.SetColor("#ff00ff")
	if err != nil {
		t.Fatalf("SetColor: %v", err)
	}
	if state.Color != "#ff00ff" || store.Color() != "#ff00ff" {
		t.Fatalf("expected color #ff00ff, got state=%q store=%q", state.Color, store.Color())
	}
	if strip.setColorCalls != 1 {
		t.Fatalf("expected strip.SetColor() to be called once, got %d", strip.setColorCalls)
	}
	if strip.lastR != 0xff || strip.lastG != 0x00 || strip.lastB != 0xff {
		t.Fatalf("expected strip color bytes (0xff, 0x00, 0xff), got (%#x, %#x, %#x)", strip.lastR, strip.lastG, strip.lastB)
	}
	if strip.onCalls != 2 {
		t.Fatalf("expected strip.On() to be called twice (initial apply + reapply after color change), got %d", strip.onCalls)
	}
}

func TestStoreSetColorStoresWithoutReapplyingWhenDisabled(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	state, err := store.SetColor("#00ff00")
	if err != nil {
		t.Fatalf("SetColor: %v", err)
	}
	if state.Color != "#00ff00" {
		t.Fatalf("expected state.Color to be #00ff00, got %q", state.Color)
	}
	if strip.setColorCalls != 1 {
		t.Fatalf("expected strip.SetColor() to be called once, got %d", strip.setColorCalls)
	}
	if strip.onCalls != 0 {
		t.Fatalf("expected strip.On() to never be called while disabled, got %d", strip.onCalls)
	}
}

func TestStoreSetColorRejectsInvalidHex(t *testing.T) {
	tests := []string{
		"",
		"ff00ff",    // missing '#'
		"#ff00f",    // too short
		"#ff00ff00", // too long
		"#gg00ff",   // non-hex digit
	}
	for _, hex := range tests {
		strip := &fakeStrip{}
		store, err := rgb.NewStore(strip, rgb.State{Enabled: false})
		if err != nil {
			t.Fatalf("NewStore: %v", err)
		}

		if _, err := store.SetColor(hex); err == nil {
			t.Fatalf("SetColor(%q): expected an error", hex)
		}
		if strip.setColorCalls != 0 {
			t.Fatalf("SetColor(%q): expected the strip to never be touched for invalid input, got %d calls", hex, strip.setColorCalls)
		}
	}
}

func TestStoreSetColorPropagatesHardwareErrorWithoutChangingState(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true, Color: "#000000"})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	strip.onErr = errBoom

	if _, err := store.SetColor("#ff00ff"); err == nil {
		t.Fatalf("expected SetColor to propagate the hardware error")
	}
	if store.Color() != "#000000" {
		t.Fatalf("expected color to remain unchanged after a failed reapply, got %q", store.Color())
	}
}

func TestStoreColorPersistsAcrossSimulatedRestart(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true, Color: "#000000"})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if _, err := store.SetColor("#123456"); err != nil {
		t.Fatalf("SetColor: %v", err)
	}

	restartedStrip := &fakeStrip{}
	restarted, err := rgb.NewStore(restartedStrip, rgb.State{Enabled: true, Color: store.Color()})
	if err != nil {
		t.Fatalf("NewStore (restart): %v", err)
	}

	if restarted.Color() != "#123456" {
		t.Fatalf("expected the color to survive the simulated restart, got %q", restarted.Color())
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
