package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/ipc"
)

var errBoom = errors.New("boom")

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func shortSocketDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "cli")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func startTestDaemon(t *testing.T, handlers map[string]ipc.Handler) string {
	t.Helper()

	path := filepath.Join(shortSocketDir(t), "pironman.sock")
	srv := ipc.NewServer(handlers)
	if err := srv.Listen(path); err != nil {
		t.Fatalf("Listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		srv.Serve(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	return path
}

func TestGatherStatusReachableInstalledActive(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"ping": func(args map[string]any) (any, error) {
			return map[string]string{"message": "pong"}, nil
		},
	})
	mgr := &fakeServiceManager{installed: true, active: true}

	info, err := gatherStatus(path, mgr)
	if err != nil {
		t.Fatalf("gatherStatus: %v", err)
	}
	if !info.Reachable {
		t.Fatalf("expected daemon to be reachable, got unreachable reason %q", info.UnreachableReason)
	}
	if !info.Installed || !info.Active {
		t.Fatalf("expected installed=true active=true, got %+v", info)
	}
	if info.Version == "" {
		t.Fatalf("expected a non-empty version")
	}
}

func TestGatherStatusUnreachableSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	mgr := &fakeServiceManager{installed: false, active: false}

	info, err := gatherStatus(path, mgr)
	if err != nil {
		t.Fatalf("expected gatherStatus to degrade gracefully, got error: %v", err)
	}
	if info.Reachable {
		t.Fatalf("expected daemon to be unreachable")
	}
	if info.UnreachableReason == "" {
		t.Fatalf("expected a clear unreachable reason")
	}
}

func TestGatherStatusPropagatesIsInstalledError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	mgr := &fakeServiceManager{isInstalledErr: errBoom}

	if _, err := gatherStatus(path, mgr); err == nil {
		t.Fatalf("expected an error when IsInstalled fails")
	}
}

func TestGatherStatusPropagatesIsActiveError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	mgr := &fakeServiceManager{installed: true, isActiveErr: errBoom}

	if _, err := gatherStatus(path, mgr); err == nil {
		t.Fatalf("expected an error when IsActive fails")
	}
}

func TestGatherDoctorReportsMissingConfigAsDefaulted(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"ping": func(args map[string]any) (any, error) { return nil, nil },
	})
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{installed: true, active: true}

	info, err := gatherDoctor(path, mgr, cfgPath)
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if !strings.Contains(info.ConfigStatus, "missing") {
		t.Fatalf("expected config status to note the file is missing, got %q", info.ConfigStatus)
	}
}

func TestGatherDoctorReportsReadableConfig(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"ping": func(args map[string]any) (any, error) { return nil, nil },
	})
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfgPath, "rgb:\n  enabled: true\n")
	mgr := &fakeServiceManager{installed: true, active: true}

	info, err := gatherDoctor(path, mgr, cfgPath)
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if info.ConfigStatus != "readable" {
		t.Fatalf("expected config status \"readable\", got %q", info.ConfigStatus)
	}
}

func TestGatherDoctorReportsMalformedConfig(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"ping": func(args map[string]any) (any, error) { return nil, nil },
	})
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfgPath, "not: [valid: yaml")
	mgr := &fakeServiceManager{installed: true, active: true}

	info, err := gatherDoctor(path, mgr, cfgPath)
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if !strings.Contains(info.ConfigStatus, "not readable") {
		t.Fatalf("expected config status to flag the parse error, got %q", info.ConfigStatus)
	}
}

func TestGatherDoctorReportsSocketPermissions(t *testing.T) {
	path := startTestDaemon(t, map[string]ipc.Handler{
		"ping": func(args map[string]any) (any, error) { return nil, nil },
	})
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{installed: true, active: true}

	info, err := gatherDoctor(path, mgr, cfgPath)
	if err != nil {
		t.Fatalf("gatherDoctor: %v", err)
	}
	if !strings.Contains(info.SocketPermissions, "660") {
		t.Fatalf("expected socket permissions to report mode 0660, got %q", info.SocketPermissions)
	}
}

func TestGatherDoctorReportsMissingSocketGracefully(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-daemon.sock")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	mgr := &fakeServiceManager{}

	info, err := gatherDoctor(path, mgr, cfgPath)
	if err != nil {
		t.Fatalf("expected gatherDoctor to degrade gracefully, got error: %v", err)
	}
	if info.SocketPermissions == "" {
		t.Fatalf("expected a non-empty (error-describing) socket permissions string")
	}
}
