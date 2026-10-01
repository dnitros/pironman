package handlers

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/hardware"
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
func (f *fakeStrip) WriteFrame(pixels []hardware.Color) error { return nil }

func startTestDaemon(t *testing.T, handlers map[string]ipc.Handler) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "handlers")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	path := filepath.Join(dir, "pironman.sock")
	srv := ipc.NewServer(handlers)
	if err := srv.Listen(path); err != nil {
		t.Fatalf("Listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		srv.Serve(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	return path
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

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

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

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

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

	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfgPath := filepath.Join(blocker, "config.yaml")

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

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

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "rgb.on", nil)
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

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

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

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

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

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

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

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

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

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

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

func TestRGBHandlersBrightnessAppliesImmediatelyWhenEnabled(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true, Color: "#ffffff", Brightness: 100})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Enabled = true
	cfg.RGB.Color = "#ffffff"
	cfg.RGB.Brightness = 100
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "rgb.brightness", map[string]any{"percent": 50})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if strip.onCalls != 2 {
		t.Fatalf("expected strip.On() to be called twice (initial apply + reapply), got %d", strip.onCalls)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.RGB.Brightness != 50 {
		t.Fatalf("expected the saved config to have rgb.brightness=50, got %d", saved.RGB.Brightness)
	}
}

func TestRGBHandlersBrightnessStoresWithoutReapplyingWhenDisabled(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false, Color: "#ffffff", Brightness: 100})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Enabled = false
	cfg.RGB.Color = "#ffffff"
	cfg.RGB.Brightness = 100
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "rgb.brightness", map[string]any{"percent": 50})
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
	if saved.RGB.Brightness != 50 {
		t.Fatalf("expected the saved config to have rgb.brightness=50, got %d", saved.RGB.Brightness)
	}
}

func TestRGBHandlersBrightnessRejectsOutOfRangeWithoutMutatingCfg(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false, Color: "#ffffff", Brightness: 100})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Brightness = 100
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "rgb.brightness", map[string]any{"percent": 150})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false for out-of-range brightness")
	}
	if strip.setColorCalls != 0 {
		t.Fatalf("expected the strip to never be touched for out-of-range brightness, got %d calls", strip.setColorCalls)
	}
	if cfg.RGB.Brightness != 100 {
		t.Fatalf("expected cfg to remain unchanged for out-of-range brightness, got %d", cfg.RGB.Brightness)
	}
}

func TestRGBHandlersBrightnessPropagatesPersistErrorWithoutMutatingCfg(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false, Color: "#ffffff", Brightness: 100})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Brightness = 100

	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfgPath := filepath.Join(blocker, "config.yaml")

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "rgb.brightness", map[string]any{"percent": 50})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false when persisting fails")
	}
	if cfg.RGB.Brightness != 100 {
		t.Fatalf("expected cfg to remain unchanged when persist fails, got %d", cfg.RGB.Brightness)
	}
	if strip.setColorCalls != 0 {
		t.Fatalf("expected the strip to never be touched when persisting fails, got %d SetColor() calls", strip.setColorCalls)
	}
}

func TestRGBBrightnessHandlerRejectsNonFinitePercent(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false, Color: "#ffffff", Brightness: 100})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Brightness = 100
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	var opMu sync.Mutex
	handler := rgbBrightnessHandler(store, &cfg, cfgPath, &opMu)

	for _, percent := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := handler(map[string]any{"percent": percent}); err == nil {
			t.Fatalf("percent=%v: expected an error", percent)
		}
	}
	if cfg.RGB.Brightness != 100 {
		t.Fatalf("expected cfg to remain unchanged, got %d", cfg.RGB.Brightness)
	}
}

func TestRGBHandlersStyleAppliesAndPersists(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false, Style: "solid"})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Style = "solid"
	cfg.RGB.Speed = 50
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "rgb.style", map[string]any{"name": "breathing", "speed": 75})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}

	saved, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if saved.RGB.Style != "breathing" || saved.RGB.Speed != 75 {
		t.Fatalf("expected persisted style=breathing speed=75, got style=%q speed=%d", saved.RGB.Style, saved.RGB.Speed)
	}
	if store.State().Style != "breathing" || store.State().Speed != 75 {
		t.Fatalf("expected store style=breathing speed=75, got %+v", store.State())
	}
}

func TestRGBHandlersStyleWithoutSpeedKeepsConfiguredSpeed(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false, Style: "solid", Speed: 33})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Style = "solid"
	cfg.RGB.Speed = 33
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "rgb.style", map[string]any{"name": "breathing"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}
	if store.State().Speed != 33 {
		t.Fatalf("expected speed to remain 33 when omitted, got %d", store.State().Speed)
	}
}

func TestRGBHandlersStyleRejectsInvalidNameWithoutMutatingCfg(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false, Style: "solid"})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Style = "solid"
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "rgb.style", map[string]any{"name": "rainbow"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false for an invalid style name")
	}
	if cfg.RGB.Style != "solid" {
		t.Fatalf("expected cfg to remain unchanged for an invalid style, got %q", cfg.RGB.Style)
	}
}

func TestRGBHandlersStyleRejectsOutOfRangeSpeedWithoutMutatingCfg(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: false, Style: "solid", Speed: 50})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg := config.Default()
	cfg.RGB.Style = "solid"
	cfg.RGB.Speed = 50
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "rgb.style", map[string]any{"name": "breathing", "speed": 150})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false for an out-of-range speed")
	}
	if cfg.RGB.Style != "solid" || cfg.RGB.Speed != 50 {
		t.Fatalf("expected cfg to remain unchanged for an invalid speed, got style=%q speed=%d", cfg.RGB.Style, cfg.RGB.Speed)
	}
}

func TestRGBHandlersBrightnessCommitsPersistedBrightnessEvenWhenStripFails(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true, Color: "#ffffff", Brightness: 100})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	strip.onErr = errors.New("spi write failed")

	cfg := config.Default()
	cfg.RGB.Enabled = true
	cfg.RGB.Color = "#ffffff"
	cfg.RGB.Brightness = 100
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	path := startTestDaemon(t, RGBHandlers(store, &cfg, cfgPath, &sync.Mutex{}))

	resp, err := ipc.Send(path, "rgb.brightness", map[string]any{"percent": 42})
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
	if saved.RGB.Brightness != 42 {
		t.Fatalf("expected the intended brightness to be persisted despite the strip failure, got %d", saved.RGB.Brightness)
	}
	if cfg.RGB.Brightness != 42 {
		t.Fatalf("expected cfg to reflect the persisted brightness despite the strip failure, got %d", cfg.RGB.Brightness)
	}
}
