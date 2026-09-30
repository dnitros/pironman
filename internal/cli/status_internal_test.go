package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/ipc"
)

func TestGatherStatusReturnsCurrentState(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"status": func(args map[string]any) (any, error) {
			return map[string]any{
				"enabled": true, "color": "#00ff00", "brightness": 80,
				"oled_awake": true, "oled_page": "mix",
				"case_fan_mode": "auto", "case_fan_relay_on": true,
				"pwm_fan_level": 2, "pwm_fan_speed_rpm": 1800,
			}, nil
		},
	})

	info, err := gatherStatus(path)
	if err != nil {
		t.Fatalf("gatherStatus: %v", err)
	}
	want := StatusInfo{
		Enabled: true, Color: "#00ff00", Brightness: 80, OLEDAwake: true, OLEDPage: "mix",
		CaseFanMode: "auto", CaseFanRelayOn: true, PWMFanLevel: 2, PWMFanSpeedRPM: 1800,
	}
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
