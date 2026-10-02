package handlers

import (
	"errors"
	"testing"

	"github.com/dnitros/pironman/internal/fan"
	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/oled"
	"github.com/dnitros/pironman/internal/rgb"
)

type fakePWMFanReader struct {
	state hardware.PWMFanState
	err   error
}

func (f *fakePWMFanReader) Read() (hardware.PWMFanState, error) {
	return f.state, f.err
}

func TestStatusHandlerReportsCurrentState(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{Enabled: true, Color: "#ff00ff", Brightness: 42, Style: rgb.StyleBreathing, Speed: 65})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	oledMachine, _ := newTestMachine(t, true)
	fanMachine, _ := newTestFanMachine(t, fan.ModeAuto)
	pwmReader := &fakePWMFanReader{state: hardware.PWMFanState{Level: 2, SpeedRPM: 1800}}

	path := startTestDaemon(t, map[string]ipc.Handler{"status": StatusHandler(store, oledMachine, fanMachine, pwmReader)})

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
	if data["style"] != rgb.StyleBreathing {
		t.Fatalf("expected style=%s, got %v", rgb.StyleBreathing, data["style"])
	}
	if data["speed"] != float64(65) {
		t.Fatalf("expected speed=65, got %v", data["speed"])
	}
	if data["oled_awake"] != true {
		t.Fatalf("expected oled_awake=true, got %v", data["oled_awake"])
	}
	if data["oled_page"] != oled.PageMix {
		t.Fatalf("expected oled_page=%s, got %v", oled.PageMix, data["oled_page"])
	}
	if data["case_fan_mode"] != fan.ModeAuto {
		t.Fatalf("expected case_fan_mode=%s, got %v", fan.ModeAuto, data["case_fan_mode"])
	}
	if data["case_fan_relay_on"] != false {
		t.Fatalf("expected case_fan_relay_on=false, got %v", data["case_fan_relay_on"])
	}
	if data["pwm_fan_level"] != float64(2) {
		t.Fatalf("expected pwm_fan_level=2, got %v", data["pwm_fan_level"])
	}
	if data["pwm_fan_speed_rpm"] != float64(1800) {
		t.Fatalf("expected pwm_fan_speed_rpm=1800, got %v", data["pwm_fan_speed_rpm"])
	}
}

func TestStatusHandlerPropagatesPWMFanReadError(t *testing.T) {
	strip := &fakeStrip{}
	store, err := rgb.NewStore(strip, rgb.State{})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	oledMachine, _ := newTestMachine(t, true)
	fanMachine, _ := newTestFanMachine(t, fan.ModeOff)
	pwmReader := &fakePWMFanReader{err: errors.New("sysfs read failed")}

	path := startTestDaemon(t, map[string]ipc.Handler{"status": StatusHandler(store, oledMachine, fanMachine, pwmReader)})

	resp, err := ipc.Send(path, "status", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false when the PWM fan read fails")
	}
}
