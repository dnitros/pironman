package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dnitros/pironman/internal/config"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, config.Default()) {
		t.Fatalf("expected defaults for a missing file, got %#v", got)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	want := config.Default()
	want.RGB.Color = "#123456"
	want.Fan.CaseFanState = "performance"

	if err := want.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch: saved %#v, loaded %#v", want, got)
	}
}

func TestSaveCreatesMissingParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "config.yaml")

	if err := config.Default().Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := config.Load(path); err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
}

func TestLoadPartialFileFillsGapsFromDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("fan:\n  case_fan_state: performance\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := config.Default()
	want.Fan.CaseFanState = "performance"
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected unset sections to keep their defaults, got %#v", got)
	}
}

func TestPathRespectsEnvVarOverride(t *testing.T) {
	t.Setenv(config.PathEnvVar, "")
	if got := config.Path(); got != config.DefaultPath {
		t.Fatalf("expected default path %q with no env var set, got %q", config.DefaultPath, got)
	}

	t.Setenv(config.PathEnvVar, "/tmp/custom-pironman-config.yaml")
	if got := config.Path(); got != "/tmp/custom-pironman-config.yaml" {
		t.Fatalf("expected env var override, got %q", got)
	}
}
