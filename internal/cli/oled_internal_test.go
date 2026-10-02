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

func TestRunOLEDImageRoundTrip(t *testing.T) {
	var gotArgs map[string]any
	path := startTestDaemon(t, map[string]ipc.Handler{
		"oled.image": func(args map[string]any) (any, error) {
			gotArgs = args
			return map[string]any{"awake": true, "page": "image", "paths": args["paths"]}, nil
		},
	})

	if err := runOLEDImage(path, []string{"a.png", "b.pbm"}, 10); err != nil {
		t.Fatalf("runOLEDImage: %v", err)
	}

	wantA, err := filepath.Abs("a.png")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	wantB, err := filepath.Abs("b.pbm")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	paths, ok := gotArgs["paths"].([]any)
	if !ok || len(paths) != 2 || paths[0] != wantA || paths[1] != wantB {
		t.Fatalf("gotArgs[\"paths\"] = %v, want [%q %q]", gotArgs["paths"], wantA, wantB)
	}
	if gotArgs["interval"] != float64(10) {
		t.Fatalf("gotArgs[\"interval\"] = %v, want 10", gotArgs["interval"])
	}
}

func TestRunOLEDImageResolvesRelativePathsToAbsolute(t *testing.T) {
	var gotArgs map[string]any
	path := startTestDaemon(t, map[string]ipc.Handler{
		"oled.image": func(args map[string]any) (any, error) {
			gotArgs = args
			return map[string]any{"awake": true, "page": "image", "paths": args["paths"]}, nil
		},
	})

	if err := runOLEDImage(path, []string{"../relative/cat.png"}, 0); err != nil {
		t.Fatalf("runOLEDImage: %v", err)
	}

	paths, ok := gotArgs["paths"].([]any)
	if !ok || len(paths) != 1 {
		t.Fatalf("gotArgs[\"paths\"] = %v, want a single path", gotArgs["paths"])
	}
	got, ok := paths[0].(string)
	if !ok || !filepath.IsAbs(got) {
		t.Fatalf("paths[0] = %v, want an absolute path (the daemon's cwd differs from the CLI's)", paths[0])
	}
}

func TestRunOLEDImageOmitsIntervalWhenNotSet(t *testing.T) {
	var gotArgs map[string]any
	path := startTestDaemon(t, map[string]ipc.Handler{
		"oled.image": func(args map[string]any) (any, error) {
			gotArgs = args
			return map[string]any{"awake": true, "page": "image", "paths": args["paths"]}, nil
		},
	})

	if err := runOLEDImage(path, []string{"a.png"}, 0); err != nil {
		t.Fatalf("runOLEDImage: %v", err)
	}
	if _, ok := gotArgs["interval"]; ok {
		t.Fatalf("expected no \"interval\" argument when --interval isn't set, got %v", gotArgs["interval"])
	}
}

func TestRunOLEDImageFailsClearlyWhenDaemonUnreachable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")

	err := runOLEDImage(path, []string{"a.png"}, 0)
	if err == nil {
		t.Fatalf("expected an error when the daemon is unreachable")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("expected error to say the daemon is unreachable, got: %v", err)
	}
}

func TestRunOLEDImagePropagatesHandlerError(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"oled.image": func(args map[string]any) (any, error) {
			return nil, errBoom
		},
	})

	err := runOLEDImage(path, []string{"a.png"}, 0)
	if err == nil {
		t.Fatalf("expected an error when the handler fails")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error to include the handler's message, got: %v", err)
	}
}
