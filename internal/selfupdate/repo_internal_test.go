package selfupdate

import (
	"runtime/debug"
	"strings"
	"testing"
)

func withBuild(t *testing.T, stamped, modulePath string) {
	t.Helper()
	oldRepo, oldRead := Repo, readBuildInfo
	Repo = stamped
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Path: modulePath}}, true
	}
	t.Cleanup(func() { Repo, readBuildInfo = oldRepo, oldRead })
}

func TestLatestURLPrefersStampedRepo(t *testing.T) {
	withBuild(t, "someone/fork", "github.com/dnitros/pironman")

	got, err := LatestURL()
	if err != nil || got != "https://api.github.com/repos/someone/fork/releases/latest" {
		t.Fatalf("LatestURL() = %q, %v; want the stamped repo", got, err)
	}
}

func TestLatestURLFallsBackToModulePath(t *testing.T) {
	withBuild(t, "", "github.com/dnitros/pironman")

	got, err := LatestURL()
	if err != nil || got != "https://api.github.com/repos/dnitros/pironman/releases/latest" {
		t.Fatalf("LatestURL() = %q, %v; want the repo from the module path", got, err)
	}
}

func TestLatestURLFailsWithoutAGitHubRepo(t *testing.T) {
	withBuild(t, "", "example.com/pironman")

	if _, err := LatestURL(); err == nil || !strings.Contains(err.Error(), "-X") {
		t.Fatalf("LatestURL() error = %v, want a hint to stamp the repo", err)
	}
}

func TestLatestURLUsesOwnerAndNameFromVersionedModulePath(t *testing.T) {
	withBuild(t, "", "github.com/dnitros/pironman/v2")

	got, err := LatestURL()
	if err != nil || got != "https://api.github.com/repos/dnitros/pironman/releases/latest" {
		t.Fatalf("LatestURL() = %q, %v; want owner/name only", got, err)
	}
}

func TestLatestURLRejectsMalformedStampedRepo(t *testing.T) {
	for _, stamped := range []string{"dnitros", "dnitros/pironman/extra", "../evil/repo", "dnitros/pironman?x=1", "owner/ name"} {
		withBuild(t, stamped, "github.com/dnitros/pironman")
		if got, err := LatestURL(); err == nil {
			t.Fatalf("LatestURL() with Repo=%q = %q, want an error", stamped, got)
		}
	}
}
