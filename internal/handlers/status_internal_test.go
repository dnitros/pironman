package handlers

import (
	"testing"

	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/oled"
	"github.com/dnitros/pironman/internal/rgb"
)

func TestStatusHandlerReportsCurrentState(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true, Color: "#ff00ff", Brightness: 42})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	machine, _ := newTestMachine(t, true)
	if err := machine.SetPage(oled.PageIPs); err != nil {
		t.Fatalf("SetPage: %v", err)
	}

	path := startTestDaemon(t, map[string]ipc.Handler{"status": StatusHandler(store, machine)})

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
	if data["oled_awake"] != true {
		t.Fatalf("expected oled_awake=true, got %v", data["oled_awake"])
	}
	if data["oled_page"] != oled.PageIPs {
		t.Fatalf("expected oled_page=%q, got %v", oled.PageIPs, data["oled_page"])
	}
}
