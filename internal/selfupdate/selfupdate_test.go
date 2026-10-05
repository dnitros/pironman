package selfupdate_test

import (
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

type fakeRelease struct {
	tag       string
	binary    []byte
	checksums string
	omitAsset string
	status    int
}

func newReleaseServer(t *testing.T, r fakeRelease) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/latest":
			if r.status != 0 {
				w.WriteHeader(r.status)
				return
			}
			var assets []string
			for _, name := range []string{selfupdate.BinaryAsset, selfupdate.ChecksumsAsset} {
				if name != r.omitAsset {
					assets = append(assets, fmt.Sprintf(`{"name":%q,"browser_download_url":%q}`, name, srv.URL+"/download/"+name))
				}
			}
			fmt.Fprintf(w, `{"tag_name":%q,"assets":[%s]}`, r.tag, strings.Join(assets, ","))
		case "/download/" + selfupdate.BinaryAsset:
			w.Write(r.binary)
		case "/download/" + selfupdate.ChecksumsAsset:
			fmt.Fprint(w, r.checksums)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func sumLine(data []byte, name string) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) + "  " + name + "\n"
}

func TestLatestReadsTagAndAssetURLs(t *testing.T) {
	srv := newReleaseServer(t, fakeRelease{tag: "v1.2.0"})

	rel, err := selfupdate.Latest(srv.Client(), srv.URL+"/latest")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if rel.Tag != "v1.2.0" {
		t.Fatalf("Tag = %q, want v1.2.0", rel.Tag)
	}
	if !strings.HasSuffix(rel.BinaryURL, "/download/"+selfupdate.BinaryAsset) || !strings.HasSuffix(rel.ChecksumsURL, "/download/"+selfupdate.ChecksumsAsset) {
		t.Fatalf("asset URLs = %q, %q", rel.BinaryURL, rel.ChecksumsURL)
	}
}

func TestLatestFailsWhenAssetMissing(t *testing.T) {
	srv := newReleaseServer(t, fakeRelease{tag: "v1.2.0", omitAsset: selfupdate.BinaryAsset})

	if _, err := selfupdate.Latest(srv.Client(), srv.URL+"/latest"); err == nil || !strings.Contains(err.Error(), selfupdate.BinaryAsset) {
		t.Fatalf("Latest error = %v, want one naming the missing %s asset", err, selfupdate.BinaryAsset)
	}
}

func TestLatestExplainsMissingRelease(t *testing.T) {
	srv := newReleaseServer(t, fakeRelease{status: http.StatusNotFound})

	_, err := selfupdate.Latest(srv.Client(), srv.URL+"/latest")
	if err == nil || !strings.Contains(err.Error(), "no published release") {
		t.Fatalf("Latest error = %v, want a hint that no release is published", err)
	}
}

func TestLatestFailsOnServerError(t *testing.T) {
	srv := newReleaseServer(t, fakeRelease{status: http.StatusInternalServerError})

	if _, err := selfupdate.Latest(srv.Client(), srv.URL+"/latest"); err == nil {
		t.Fatalf("Latest succeeded on a 500, want an error")
	}
}

func TestDownloadReturnsVerifiedBinary(t *testing.T) {
	bin := []byte("new pironman binary")
	srv := newReleaseServer(t, fakeRelease{tag: "v1.2.0", binary: bin, checksums: sumLine([]byte("other"), "other") + sumLine(bin, selfupdate.BinaryAsset)})
	rel, err := selfupdate.Latest(srv.Client(), srv.URL+"/latest")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}

	got, err := selfupdate.Download(srv.Client(), rel)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if string(got) != string(bin) {
		t.Fatalf("Download = %q, want %q", got, bin)
	}
}

func TestDownloadRejectsBadOrMissingChecksum(t *testing.T) {
	bin := []byte("new pironman binary")
	cases := map[string]string{
		"mismatch": sumLine([]byte("tampered"), selfupdate.BinaryAsset),
		"missing":  sumLine(bin, "some-other-file"),
	}
	for name, checksums := range cases {
		srv := newReleaseServer(t, fakeRelease{tag: "v1.2.0", binary: bin, checksums: checksums})
		rel, err := selfupdate.Latest(srv.Client(), srv.URL+"/latest")
		if err != nil {
			t.Fatalf("%s: Latest: %v", name, err)
		}
		if _, err := selfupdate.Download(srv.Client(), rel); err == nil {
			t.Fatalf("%s: Download succeeded, want a checksum error", name)
		}
	}
}

func TestReplaceSwapsFileAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pironman")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := selfupdate.Replace(path, []byte("new")); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Fatalf("contents = %q, want new", got)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, want 0755", info.Mode().Perm())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("dir has %d entries, want only the replaced binary", len(entries))
	}
}

func TestReplaceFailureLeavesOriginalUntouched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pironman")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if err := selfupdate.Replace(path, []byte("new")); err == nil {
		t.Fatalf("Replace into a read-only dir succeeded, want an error")
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Fatalf("contents = %q, want the original left in place", got)
	}
}

func TestRepoURL(t *testing.T) {
	const want = "https://api.github.com/repos/dnitros/pironman/releases/latest"
	cases := []struct {
		name, stamped, modulePath, want string
	}{
		{"stamped repo wins", "someone/fork", "github.com/dnitros/pironman", "https://api.github.com/repos/someone/fork/releases/latest"},
		{"falls back to module path", "", "github.com/dnitros/pironman", want},
		{"versioned module path keeps owner/name", "", "github.com/dnitros/pironman/v2", want},
	}
	for _, c := range cases {
		got, err := selfupdate.RepoURL(c.stamped, c.modulePath)
		if err != nil || got != c.want {
			t.Fatalf("%s: RepoURL = %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}

func TestRepoURLRejectsMissingOrMalformedRepo(t *testing.T) {
	if _, err := selfupdate.RepoURL("", "example.com/pironman"); err == nil || !strings.Contains(err.Error(), "-X") {
		t.Fatalf("RepoURL without a GitHub repo: error = %v, want a hint to stamp the repo", err)
	}
	for _, stamped := range []string{"dnitros", "dnitros/pironman/extra", "../evil/repo", "dnitros/pironman?x=1", "owner/ name"} {
		if got, err := selfupdate.RepoURL(stamped, "github.com/dnitros/pironman"); err == nil {
			t.Fatalf("RepoURL(%q) = %q, want an error", stamped, got)
		}
	}
}
