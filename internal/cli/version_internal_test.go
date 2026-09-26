package cli

import (
	"runtime/debug"
	"testing"
)

func TestFormatVersionShortensRevision(t *testing.T) {
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "abcdef1234567890"},
	}}
	if got, want := formatVersion(info), "abcdef1"; got != want {
		t.Fatalf("formatVersion() = %q, want %q", got, want)
	}
}

func TestFormatVersionMarksDirtyTree(t *testing.T) {
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "abcdef1234567890"},
		{Key: "vcs.modified", Value: "true"},
	}}
	if got, want := formatVersion(info), "abcdef1+dirty"; got != want {
		t.Fatalf("formatVersion() = %q, want %q", got, want)
	}
}

func TestFormatVersionCleanTreeHasNoSuffix(t *testing.T) {
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "abcdef1234567890"},
		{Key: "vcs.modified", Value: "false"},
	}}
	if got, want := formatVersion(info), "abcdef1"; got != want {
		t.Fatalf("formatVersion() = %q, want %q", got, want)
	}
}

func TestFormatVersionDoesNotPanicOnShortRevision(t *testing.T) {
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "abc"},
	}}
	if got, want := formatVersion(info), "abc"; got != want {
		t.Fatalf("formatVersion() = %q, want %q", got, want)
	}
}

func TestFormatVersionFallsBackWithoutRevision(t *testing.T) {
	info := &debug.BuildInfo{}
	if got, want := formatVersion(info), "unknown"; got != want {
		t.Fatalf("formatVersion() = %q, want %q", got, want)
	}
}

func TestVersionStringFallsBackWhenBuildInfoUnavailable(t *testing.T) {
	old := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	t.Cleanup(func() { readBuildInfo = old })

	if got, want := versionString(), "unknown"; got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}

func TestVersionStringUsesBuildInfoRevision(t *testing.T) {
	old := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "deadbee123456"},
		}}, true
	}
	t.Cleanup(func() { readBuildInfo = old })

	if got, want := versionString(), "deadbee"; got != want {
		t.Fatalf("versionString() = %q, want %q", got, want)
	}
}
