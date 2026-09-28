package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/rgb"
)

type fakeStrip struct {
	onCalls, offCalls int
}

func (f *fakeStrip) On() error  { f.onCalls++; return nil }
func (f *fakeStrip) Off() error { f.offCalls++; return nil }

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
