package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/rgb"
)

func TestStatusHandlerReportsCurrentState(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true, Color: "#ff00ff", Brightness: 42})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	path := startTestDaemon(t, map[string]ipc.Handler{"status": statusHandler(store)})

	resp, err := ipc.Send(path, "status", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %#v", resp.Data)
	}
	if data["enabled"] != true {
		t.Fatalf("expected enabled=true, got %v", data["enabled"])
	}
	if data["color"] != "#ff00ff" {
		t.Fatalf("expected color=#ff00ff, got %v", data["color"])
	}
	if data["brightness"] != float64(42) {
		t.Fatalf("expected brightness=42, got %v", data["brightness"])
	}
}

func TestGatherStatusReturnsCurrentState(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"status": func(args map[string]any) (any, error) {
			return map[string]any{"enabled": true, "color": "#00ff00", "brightness": 80}, nil
		},
	})

	info, err := gatherStatus(path)
	if err != nil {
		t.Fatalf("gatherStatus: %v", err)
	}
	want := StatusInfo{Enabled: true, Color: "#00ff00", Brightness: 80}
	if info != want {
		t.Fatalf("gatherStatus() = %+v, want %+v", info, want)
	}
}

func TestGatherStatusFailsClearlyWhenDaemonUnreachable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")

	_, err := gatherStatus(path)
	if err == nil {
		t.Fatalf("expected an error when the daemon is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected error to say the daemon is unreachable, got: %v", err)
	}
}

func TestGatherStatusPropagatesHandlerError(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"status": func(args map[string]any) (any, error) {
			return nil, errBoom
		},
	})

	_, err := gatherStatus(path)
	if err == nil {
		t.Fatalf("expected an error when the handler fails")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include the handler's message, got: %v", err)
	}
}
