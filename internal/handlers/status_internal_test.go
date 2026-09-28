package handlers

import (
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

	path := startTestDaemon(t, map[string]ipc.Handler{"status": StatusHandler(store)})

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
