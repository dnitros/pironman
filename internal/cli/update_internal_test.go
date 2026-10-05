package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/selfupdate"
)

func newUpdateServer(t *testing.T, tag string, bin []byte, checksumOf []byte) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch r.URL.Path {
		case "/latest":
			fmt.Fprintf(w, `{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":%q},{"name":%q,"browser_download_url":%q}]}`,
				tag, selfupdate.BinaryAsset, srv.URL+"/bin", selfupdate.ChecksumsAsset, srv.URL+"/sums")
		case "/bin":
			w.Write(bin)
		case "/sums":
			sum := sha256.Sum256(checksumOf)
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), selfupdate.BinaryAsset)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func newUpdateTarget(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pironman")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestUpdateInstallsNewerReleaseAndRestartsService(t *testing.T) {
	asRoot(t)
	srv, _ := newUpdateServer(t, "v1.2.0", []byte("new"), []byte("new"))
	path := newUpdateTarget(t)
	mgr := &fakeServiceManager{installed: true, active: true}
	var out bytes.Buffer

	err := runUpdate(updateEnv{client: srv.Client(), latestURL: srv.URL + "/latest", binPath: path, current: "v1.1.0", mgr: mgr}, false, &out)

	if err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if got := readFile(t, path); got != "new" {
		t.Fatalf("binary = %q, want new", got)
	}
	if !mgr.restartCalled {
		t.Fatalf("expected the running service to be restarted")
	}
	if !strings.Contains(out.String(), "v1.1.0 → v1.2.0") {
		t.Fatalf("output = %q, want old → new versions", out.String())
	}
}

func TestUpdateLeavesStoppedServiceStopped(t *testing.T) {
	asRoot(t)
	srv, _ := newUpdateServer(t, "v1.2.0", []byte("new"), []byte("new"))
	mgr := &fakeServiceManager{installed: true, active: false}

	if err := runUpdate(updateEnv{client: srv.Client(), latestURL: srv.URL + "/latest", binPath: newUpdateTarget(t), current: "v1.1.0", mgr: mgr}, false, &bytes.Buffer{}); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if mgr.restartCalled {
		t.Fatalf("restarted a stopped service, want it left stopped")
	}
}

func TestUpdateAlreadyUpToDateChangesNothing(t *testing.T) {
	asRoot(t)
	srv, _ := newUpdateServer(t, "v1.2.0", []byte("new"), []byte("new"))
	path := newUpdateTarget(t)
	mgr := &fakeServiceManager{installed: true, active: true}
	var out bytes.Buffer

	if err := runUpdate(updateEnv{client: srv.Client(), latestURL: srv.URL + "/latest", binPath: path, current: "v1.2.0", mgr: mgr}, false, &out); err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if readFile(t, path) != "old" || mgr.restartCalled {
		t.Fatalf("binary or service changed while already up to date")
	}
	if !strings.Contains(out.String(), "already up to date") {
		t.Fatalf("output = %q, want 'already up to date'", out.String())
	}
}

func TestUpdateChecksumMismatchChangesNothing(t *testing.T) {
	asRoot(t)
	srv, _ := newUpdateServer(t, "v1.2.0", []byte("tampered"), []byte("new"))
	path := newUpdateTarget(t)
	mgr := &fakeServiceManager{installed: true, active: true}

	if err := runUpdate(updateEnv{client: srv.Client(), latestURL: srv.URL + "/latest", binPath: path, current: "v1.1.0", mgr: mgr}, false, &bytes.Buffer{}); err == nil {
		t.Fatalf("runUpdate succeeded with a bad checksum, want an error")
	}
	if readFile(t, path) != "old" || mgr.restartCalled {
		t.Fatalf("binary or service changed after a checksum mismatch")
	}
}

func TestUpdateWithoutRootFailsBeforeAnyRequest(t *testing.T) {
	asNonRoot(t)
	srv, hits := newUpdateServer(t, "v1.2.0", []byte("new"), []byte("new"))

	err := runUpdate(updateEnv{client: srv.Client(), latestURL: srv.URL + "/latest", binPath: newUpdateTarget(t), current: "v1.1.0", mgr: &fakeServiceManager{}}, false, &bytes.Buffer{})

	if err == nil || !strings.Contains(err.Error(), "sudo") {
		t.Fatalf("error = %v, want a sudo hint", err)
	}
	if *hits != 0 {
		t.Fatalf("made %d requests before the root check, want none", *hits)
	}
}

func TestUpdateCheckReportsWithoutRootOrInstalling(t *testing.T) {
	asNonRoot(t)
	srv, _ := newUpdateServer(t, "v1.2.0", []byte("new"), []byte("new"))
	path := newUpdateTarget(t)
	mgr := &fakeServiceManager{installed: true, active: true}
	var out bytes.Buffer

	if err := runUpdate(updateEnv{client: srv.Client(), latestURL: srv.URL + "/latest", binPath: path, current: "v1.1.0", mgr: mgr}, true, &out); err != nil {
		t.Fatalf("runUpdate --check: %v", err)
	}
	if readFile(t, path) != "old" || mgr.restartCalled {
		t.Fatalf("--check changed the binary or service")
	}
	if !strings.Contains(out.String(), "v1.1.0 → v1.2.0") {
		t.Fatalf("output = %q, want the available update", out.String())
	}
}

func TestUpdateRestartFailureSaysBinaryWasUpdated(t *testing.T) {
	asRoot(t)
	srv, _ := newUpdateServer(t, "v1.2.0", []byte("new"), []byte("new"))
	path := newUpdateTarget(t)
	mgr := &fakeServiceManager{installed: true, active: true, restartErr: fmt.Errorf("dbus timeout")}

	err := runUpdate(updateEnv{client: srv.Client(), latestURL: srv.URL + "/latest", binPath: path, current: "v1.1.0", mgr: mgr}, false, &bytes.Buffer{})

	if err == nil || !strings.Contains(err.Error(), "updated v1.1.0 → v1.2.0") || !strings.Contains(err.Error(), "systemctl restart pironman") {
		t.Fatalf("error = %v, want it to say the binary was updated and how to restart", err)
	}
	if readFile(t, path) != "new" {
		t.Fatalf("binary should already be replaced when only the restart fails")
	}
}
