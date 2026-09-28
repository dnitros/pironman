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
	onCalls, offCalls int
	onErr, offErr     error
}

func (f *fakeStrip) On() error  { f.onCalls++; return f.onErr }
func (f *fakeStrip) Off() error { f.offCalls++; return f.offErr }

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
