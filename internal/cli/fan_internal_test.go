package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/ipc"
)

func TestRunFanModeRoundTrip(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"fan.mode": func(args map[string]any) (any, error) {
			if args["name"] != "balanced" {
				t.Fatalf("expected name=balanced, got %v", args["name"])
			}
			return map[string]any{"mode": "balanced", "relay_on": false}, nil
		},
	})

	if err := runFanMode(path, "balanced"); err != nil {
		t.Fatalf("runFanMode: %v", err)
	}
}

func TestRunFanModeFailsClearlyWhenDaemonUnreachable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")

	err := runFanMode(path, "balanced")
	if err == nil {
		t.Fatalf("expected an error when the daemon is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected error to say the daemon is unreachable, got: %v", err)
	}
}

func TestRunFanModePropagatesHandlerError(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"fan.mode": func(args map[string]any) (any, error) {
			return nil, errBoom
		},
	})

	err := runFanMode(path, "bogus")
	if err == nil {
		t.Fatalf("expected an error when the handler fails")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include the handler's message, got: %v", err)
	}
}
