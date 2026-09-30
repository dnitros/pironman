package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/ipc"
)

func TestRunFanSetRoundTrip(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"fan.auto": func(args map[string]any) (any, error) {
			return map[string]any{"mode": "auto", "relay_on": false}, nil
		},
	})

	if err := runFanSet(path, "auto"); err != nil {
		t.Fatalf("runFanSet: %v", err)
	}
}

func TestRunFanSetFailsClearlyWhenDaemonUnreachable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")

	err := runFanSet(path, "on")
	if err == nil {
		t.Fatalf("expected an error when the daemon is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected error to say the daemon is unreachable, got: %v", err)
	}
}

func TestRunFanSetPropagatesHandlerError(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"fan.on": func(args map[string]any) (any, error) {
			return nil, errBoom
		},
	})

	err := runFanSet(path, "on")
	if err == nil {
		t.Fatalf("expected an error when the handler fails")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include the handler's message, got: %v", err)
	}
}
