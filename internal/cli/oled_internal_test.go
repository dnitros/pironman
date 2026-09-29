package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/ipc"
)

func TestRunOLEDSetRoundTrip(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"oled.on": func(args map[string]any) (any, error) {
			return map[string]bool{"awake": true}, nil
		},
	})

	if err := runOLEDSet(path, "on"); err != nil {
		t.Fatalf("runOLEDSet: %v", err)
	}
}

func TestRunOLEDSetFailsClearlyWhenDaemonUnreachable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")

	err := runOLEDSet(path, "on")
	if err == nil {
		t.Fatalf("expected an error when the daemon is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected error to say the daemon is unreachable, got: %v", err)
	}
}

func TestRunOLEDSetPropagatesHandlerError(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"oled.on": func(args map[string]any) (any, error) {
			return nil, errBoom
		},
	})

	err := runOLEDSet(path, "on")
	if err == nil {
		t.Fatalf("expected an error when the handler fails")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include the handler's message, got: %v", err)
	}
}

func TestRunOLEDPageRoundTrip(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"oled.page": func(args map[string]any) (any, error) {
			return map[string]any{"awake": true, "page": args["page"]}, nil
		},
	})

	if err := runOLEDPage(path, "mix"); err != nil {
		t.Fatalf("runOLEDPage: %v", err)
	}
}

func TestRunOLEDPageFailsClearlyWhenDaemonUnreachable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")

	err := runOLEDPage(path, "next")
	if err == nil {
		t.Fatalf("expected an error when the daemon is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected error to say the daemon is unreachable, got: %v", err)
	}
}

func TestRunOLEDPagePropagatesHandlerError(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"oled.page": func(args map[string]any) (any, error) {
			return nil, errBoom
		},
	})

	err := runOLEDPage(path, "not-a-page")
	if err == nil {
		t.Fatalf("expected an error when the handler fails")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include the handler's message, got: %v", err)
	}
}
