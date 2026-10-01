package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/ipc"
)

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

func TestRunRGBColorRoundTrip(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"rgb.color": func(args map[string]any) (any, error) {
			return map[string]string{"color": args["hex"].(string)}, nil
		},
	})

	if err := runRGBColor(path, "#ff00ff"); err != nil {
		t.Fatalf("runRGBColor: %v", err)
	}
}

func TestRunRGBColorFailsClearlyWhenDaemonUnreachable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")

	err := runRGBColor(path, "#ff00ff")
	if err == nil {
		t.Fatalf("expected an error when the daemon is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected error to say the daemon is unreachable, got: %v", err)
	}
}

func TestRunRGBColorPropagatesHandlerError(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"rgb.color": func(args map[string]any) (any, error) {
			return nil, errBoom
		},
	})

	err := runRGBColor(path, "#ff00ff")
	if err == nil {
		t.Fatalf("expected an error when the handler fails")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include the handler's message, got: %v", err)
	}
}

func TestRGBBrightnessCommandAcceptsNegativeValue(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"rgb", "brightness", "-5"})
	t.Setenv(ipc.SocketPathEnvVar, filepath.Join(t.TempDir(), "no-such-daemon.sock"))

	err := root.Execute()
	if err == nil {
		t.Fatalf("expected an error since no daemon is running")
	}
	if strings.Contains(err.Error(), "shorthand") || strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("expected \"-5\" to be parsed as the brightness value, not a flag, got: %v", err)
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected a daemon-unreachable error once parsing succeeds, got: %v", err)
	}
}

func TestRGBBrightnessCommandStillShowsHelp(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"rgb", "brightness", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected --help to succeed, got: %v", err)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("expected usage text, got: %q", out.String())
	}
}

func TestRunRGBBrightnessRoundTrip(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"rgb.brightness": func(args map[string]any) (any, error) {
			return map[string]int{"brightness": int(args["percent"].(float64))}, nil
		},
	})

	if err := runRGBBrightness(path, 50); err != nil {
		t.Fatalf("runRGBBrightness: %v", err)
	}
}

func TestRunRGBBrightnessFailsClearlyWhenDaemonUnreachable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")

	err := runRGBBrightness(path, 50)
	if err == nil {
		t.Fatalf("expected an error when the daemon is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected error to say the daemon is unreachable, got: %v", err)
	}
}

func TestRunRGBStyleRoundTrip(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"rgb.style": func(args map[string]any) (any, error) {
			return map[string]any{"style": args["name"]}, nil
		},
	})

	if err := runRGBStyle(path, "breathing", nil); err != nil {
		t.Fatalf("runRGBStyle: %v", err)
	}
}

func TestRunRGBStyleOmitsSpeedWhenNil(t *testing.T) {
	var gotArgs map[string]any
	path := startTestDaemon(t, map[string]ipc.Handler{
		"rgb.style": func(args map[string]any) (any, error) {
			gotArgs = args
			return map[string]any{"style": args["name"]}, nil
		},
	})

	if err := runRGBStyle(path, "breathing", nil); err != nil {
		t.Fatalf("runRGBStyle: %v", err)
	}
	if _, present := gotArgs["speed"]; present {
		t.Fatalf("expected no \"speed\" argument when speed is nil, got %v", gotArgs["speed"])
	}
}

func TestRunRGBStylePassesSpeedWhenSet(t *testing.T) {
	var gotArgs map[string]any
	path := startTestDaemon(t, map[string]ipc.Handler{
		"rgb.style": func(args map[string]any) (any, error) {
			gotArgs = args
			return map[string]any{"style": args["name"]}, nil
		},
	})

	speed := 75
	if err := runRGBStyle(path, "breathing", &speed); err != nil {
		t.Fatalf("runRGBStyle: %v", err)
	}
	if gotArgs["speed"] != float64(75) {
		t.Fatalf("expected speed=75, got %v", gotArgs["speed"])
	}
}

func TestRunRGBStyleFailsClearlyWhenDaemonUnreachable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")

	err := runRGBStyle(path, "breathing", nil)
	if err == nil {
		t.Fatalf("expected an error when the daemon is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected error to say the daemon is unreachable, got: %v", err)
	}
}

func TestRunRGBStylePropagatesHandlerError(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"rgb.style": func(args map[string]any) (any, error) {
			return nil, errBoom
		},
	})

	err := runRGBStyle(path, "breathing", nil)
	if err == nil {
		t.Fatalf("expected an error when the handler fails")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include the handler's message, got: %v", err)
	}
}

func TestRGBStyleCommandAcceptsSpeedFlag(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"rgb", "style", "breathing", "--speed", "75"})
	t.Setenv(ipc.SocketPathEnvVar, filepath.Join(t.TempDir(), "no-such-daemon.sock"))

	err := root.Execute()
	if err == nil {
		t.Fatalf("expected an error since no daemon is running")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected a daemon-unreachable error once parsing succeeds, got: %v", err)
	}
}

func TestRunRGBBrightnessPropagatesHandlerError(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"rgb.brightness": func(args map[string]any) (any, error) {
			return nil, errBoom
		},
	})

	err := runRGBBrightness(path, 50)
	if err == nil {
		t.Fatalf("expected an error when the handler fails")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include the handler's message, got: %v", err)
	}
}
