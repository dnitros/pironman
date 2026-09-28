package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/rgb"
)

type fakeStrip struct {
	onCalls, offCalls, setColorCalls int
	onErr, offErr                    error
	lastR, lastG, lastB              byte
}

func (f *fakeStrip) On() error  { f.onCalls++; return f.onErr }
func (f *fakeStrip) Off() error { f.offCalls++; return f.offErr }
func (f *fakeStrip) SetColor(r, g, b byte) {
	f.setColorCalls++
	f.lastR, f.lastG, f.lastB = r, g, b
}

func TestRGBHandlersOnPersistsStateAndCallsStrip(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Enabled = false
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, rgbHandlers(store, &cfg, cfgPath))

	resp, err := ipc.Send(path, "rgb.on", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if strip.onCalls != 1 {
		t.Fatalf("expected strip.On() to be called once, got %d", strip.onCalls)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !saved.RGB.Enabled {
		t.Fatalf("expected the saved config to have rgb.enabled=true")
	}
}

func TestRGBHandlersOffPersistsStateAndCallsStrip(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Enabled = true
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, rgbHandlers(store, &cfg, cfgPath))

	resp, err := ipc.Send(path, "rgb.off", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if strip.offCalls != 1 {
		t.Fatalf("expected strip.Off() to be called once, got %d", strip.offCalls)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.RGB.Enabled {
		t.Fatalf("expected the saved config to have rgb.enabled=false")
	}
}

func TestRGBHandlersPropagatesPersistErrorWithoutMutatingCfg(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Enabled = false

	// A regular file in place of the config directory makes cfg.Save's
	// os.MkdirAll fail, so the persist step returns an error.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfgPath := filepath.Join(blocker, "config.yaml")

	path := startTestDaemon(t, rgbHandlers(store, &cfg, cfgPath))

	resp, err := ipc.Send(path, "rgb.on", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false when persisting fails")
	}
	if cfg.RGB.Enabled {
		t.Fatalf("expected cfg to remain unchanged when persist fails, got enabled=true")
	}
	if strip.onCalls != 0 {
		t.Fatalf("expected the strip to never be touched when persisting fails, got %d On() calls", strip.onCalls)
	}
}

func TestRGBHandlersCommitsPersistedStateEvenWhenStripFails(t *testing.T) {
	strip := &fakeStrip{onErr: errors.New("spi write failed")}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Enabled = false
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, rgbHandlers(store, &cfg, cfgPath))

	resp, err := ipc.Send(path, "rgb.on", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false when the strip write fails")
	}

	// A restart's rgb.NewStore reapplies whatever is on disk, so the intended
	// state must be persisted even though the strip write itself failed.
	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !saved.RGB.Enabled {
		t.Fatalf("expected the intended state to be persisted despite the strip failure")
	}
	if !cfg.RGB.Enabled {
		t.Fatalf("expected cfg to reflect the persisted state despite the strip failure")
	}
}

func TestRGBHandlersConcurrentCallsKeepConfigInSyncWithStore(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Enabled = false
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, rgbHandlers(store, &cfg, cfgPath))

	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		cmd := "rgb.on"
		if i%2 == 0 {
			cmd = "rgb.off"
		}
		go func(cmd string) {
			defer wg.Done()
			ipc.Send(path, cmd, nil)
		}(cmd)
	}
	wg.Wait()

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.RGB.Enabled != store.Enabled() {
		t.Fatalf("persisted config (enabled=%v) disagrees with the store's final state (enabled=%v) after concurrent calls", saved.RGB.Enabled, store.Enabled())
	}
}

func TestRGBHandlersColorAppliesImmediatelyWhenEnabled(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true, Color: "#000000"})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Enabled = true
	cfg.RGB.Color = "#000000"
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, rgbHandlers(store, &cfg, cfgPath))

	resp, err := ipc.Send(path, "rgb.color", map[string]any{"hex": "#ff00ff"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if strip.setColorCalls != 1 {
		t.Fatalf("expected strip.SetColor() to be called once, got %d", strip.setColorCalls)
	}
	if strip.onCalls != 2 {
		t.Fatalf("expected strip.On() to be called twice (initial apply + reapply), got %d", strip.onCalls)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.RGB.Color != "#ff00ff" {
		t.Fatalf("expected the saved config to have rgb.color=#ff00ff, got %q", saved.RGB.Color)
	}
}

func TestRGBHandlersColorStoresWithoutReapplyingWhenDisabled(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false, Color: "#000000"})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Enabled = false
	cfg.RGB.Color = "#000000"
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, rgbHandlers(store, &cfg, cfgPath))

	resp, err := ipc.Send(path, "rgb.color", map[string]any{"hex": "#00ff00"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if strip.onCalls != 0 {
		t.Fatalf("expected strip.On() to never be called while disabled, got %d", strip.onCalls)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.RGB.Color != "#00ff00" {
		t.Fatalf("expected the saved config to have rgb.color=#00ff00, got %q", saved.RGB.Color)
	}
}

func TestRGBHandlersColorRejectsInvalidHexWithoutMutatingCfg(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false, Color: "#000000"})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Color = "#000000"
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, rgbHandlers(store, &cfg, cfgPath))

	resp, err := ipc.Send(path, "rgb.color", map[string]any{"hex": "not-a-color"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false for invalid hex")
	}
	if strip.setColorCalls != 0 {
		t.Fatalf("expected the strip to never be touched for invalid hex, got %d calls", strip.setColorCalls)
	}
	if cfg.RGB.Color != "#000000" {
		t.Fatalf("expected cfg to remain unchanged for invalid hex, got %q", cfg.RGB.Color)
	}
}

func TestRGBHandlersColorCommitsPersistedColorEvenWhenStripFails(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true, Color: "#000000"})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	strip.onErr = errors.New("spi write failed")

	cfg := config.Default()
	cfg.RGB.Enabled = true
	cfg.RGB.Color = "#000000"
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, rgbHandlers(store, &cfg, cfgPath))

	resp, err := ipc.Send(path, "rgb.color", map[string]any{"hex": "#123456"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false when the strip write fails")
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.RGB.Color != "#123456" {
		t.Fatalf("expected the intended color to be persisted despite the strip failure, got %q", saved.RGB.Color)
	}
	if cfg.RGB.Color != "#123456" {
		t.Fatalf("expected cfg to reflect the persisted color despite the strip failure, got %q", cfg.RGB.Color)
	}
}

func TestRunRGBSetRoundTrip(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"rgb.on": func(args map[string]any) (any, error) {
			return map[string]bool{"enabled": true}, nil
		},
	})

	if err := runRGBSet(path, "on"); err != nil {
		t.Fatalf("runRGBSet: %v", err)
	}
}

func TestRunRGBSetFailsClearlyWhenDaemonUnreachable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")

	err := runRGBSet(path, "on")
	if err == nil {
		t.Fatalf("expected an error when the daemon is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected error to say the daemon is unreachable, got: %v", err)
	}
}

func TestRunRGBSetPropagatesHandlerError(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"rgb.on": func(args map[string]any) (any, error) {
			return nil, errBoom
		},
	})

	err := runRGBSet(path, "on")
	if err == nil {
		t.Fatalf("expected an error when the handler fails")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include the handler's message, got: %v", err)
	}
}

func TestRunRGBColorRoundTrip(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"rgb.color": func(args map[string]any) (any, error) {
			return map[string]string{"color": args["hex"].(string)}, nil
		},
	})

	if err := runRGBColor(path, "#ff00ff"); err != nil {
		t.Fatalf("runRGBColor: %v", err)
	}
}

func TestRunRGBColorFailsClearlyWhenDaemonUnreachable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")

	err := runRGBColor(path, "#ff00ff")
	if err == nil {
		t.Fatalf("expected an error when the daemon is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected error to say the daemon is unreachable, got: %v", err)
	}
}

func TestRunRGBColorPropagatesHandlerError(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"rgb.color": func(args map[string]any) (any, error) {
			return nil, errBoom
		},
	})

	err := runRGBColor(path, "#ff00ff")
	if err == nil {
		t.Fatalf("expected an error when the handler fails")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include the handler's message, got: %v", err)
	}
}
