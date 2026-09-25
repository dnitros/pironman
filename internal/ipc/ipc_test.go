package ipc_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/dnitros/pironman/internal/ipc"
)

// shortSocketDir returns a temp dir outside t.TempDir(), which embeds the
// test name and can push the socket path past macOS's ~104-byte sun_path
// limit for tests with longer names.
func shortSocketDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "ipc")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func startTestServer(t *testing.T, handlers map[string]ipc.Handler) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "pironman.sock")
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

func TestPingRoundTrip(t *testing.T) {
	path := startTestServer(t, map[string]ipc.Handler{
		"ping": func(args map[string]any) (any, error) {
			return map[string]string{"message": "pong"}, nil
		},
	})

	resp, err := ipc.Send(path, "ping", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true, got error %q", resp.Error)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok || data["message"] != "pong" {
		t.Fatalf("expected data.message=pong, got %#v", resp.Data)
	}
}

func TestListenSetsSocketPermissions(t *testing.T) {
	path := filepath.Join(shortSocketDir(t), "pironman.sock")

	srv := ipc.NewServer(nil)
	if err := srv.Listen(path); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { srv.Close() })

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o660 {
		t.Fatalf("expected socket mode 0660 per ADR-0002, got %o", got)
	}
}

func TestListenRemovesActuallyStaleSocket(t *testing.T) {
	path := filepath.Join(shortSocketDir(t), "pironman.sock")

	// Simulate a leftover socket file with no live listener behind it, e.g.
	// the daemon crashed without a clean shutdown.
	if err := os.WriteFile(path, nil, 0o660); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	srv := ipc.NewServer(nil)
	if err := srv.Listen(path); err != nil {
		t.Fatalf("expected Listen to clear a genuinely stale socket, got: %v", err)
	}
	t.Cleanup(func() { srv.Close() })
}

func TestListenRefusesLiveSocket(t *testing.T) {
	path := filepath.Join(shortSocketDir(t), "pironman.sock")

	first := ipc.NewServer(nil)
	if err := first.Listen(path); err != nil {
		t.Fatalf("Listen (first): %v", err)
	}
	t.Cleanup(func() { first.Close() })

	second := ipc.NewServer(nil)
	if err := second.Listen(path); err == nil {
		t.Fatalf("expected Listen to refuse a socket a live daemon is already using")
	}
}

func TestUnknownCommand(t *testing.T) {
	path := startTestServer(t, map[string]ipc.Handler{})

	resp, err := ipc.Send(path, "bogus", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false for unknown command")
	}
	if resp.Error == "" {
		t.Fatalf("expected a non-empty error message")
	}
}

func TestMalformedJSON(t *testing.T) {
	path := startTestServer(t, map[string]ipc.Handler{})

	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("not json\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	var resp ipc.Response
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected ok=false for malformed JSON")
	}
	if resp.Error == "" {
		t.Fatalf("expected a non-empty error message")
	}
}
