package hardware

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePWMFanIntTrimsWhitespace(t *testing.T) {
	got, err := parsePWMFanInt([]byte("3\n"))
	if err != nil {
		t.Fatalf("parsePWMFanInt: %v", err)
	}
	if got != 3 {
		t.Fatalf("got %d, want 3", got)
	}
}

func TestParsePWMFanIntRejectsMalformedInput(t *testing.T) {
	if _, err := parsePWMFanInt([]byte("not-a-number\n")); err == nil {
		t.Fatalf("expected an error for malformed input")
	}
}

func TestResolvePWMFanInputPathFindsHwmonMatch(t *testing.T) {
	dir := t.TempDir()
	hwmonDir := filepath.Join(dir, "hwmon3")
	if err := os.MkdirAll(hwmonDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	want := filepath.Join(hwmonDir, "fan1_input")
	if err := os.WriteFile(want, []byte("2100\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := resolvePWMFanInputPath(filepath.Join(dir, "hwmon*", "fan1_input"))
	if err != nil {
		t.Fatalf("resolvePWMFanInputPath: %v", err)
	}
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestResolvePWMFanInputPathErrorsWhenNoMatch(t *testing.T) {
	dir := t.TempDir()
	if _, err := resolvePWMFanInputPath(filepath.Join(dir, "hwmon*", "fan1_input")); err == nil {
		t.Fatalf("expected an error when no hwmon path matches")
	}
}

func TestSysPWMFanReaderReadReportsLevelAndSpeed(t *testing.T) {
	dir := t.TempDir()
	coolingStatePath := filepath.Join(dir, "cur_state")
	if err := os.WriteFile(coolingStatePath, []byte("2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	hwmonDir := filepath.Join(dir, "hwmon0")
	if err := os.MkdirAll(hwmonDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hwmonDir, "fan1_input"), []byte("1800\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reader := NewSysPWMFanReader(coolingStatePath, filepath.Join(dir, "hwmon*", "fan1_input"))
	got, err := reader.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	want := PWMFanState{Level: 2, SpeedRPM: 1800}
	if got != want {
		t.Fatalf("Read() = %+v, want %+v", got, want)
	}
}

func TestSysPWMFanReaderReadPropagatesMissingCoolingStateError(t *testing.T) {
	dir := t.TempDir()
	reader := NewSysPWMFanReader(filepath.Join(dir, "no-such-file"), filepath.Join(dir, "hwmon*", "fan1_input"))
	if _, err := reader.Read(); err == nil {
		t.Fatalf("expected an error when the cooling-state file is missing")
	}
}

func TestSysPWMFanReaderReadPropagatesMissingHwmonError(t *testing.T) {
	dir := t.TempDir()
	coolingStatePath := filepath.Join(dir, "cur_state")
	if err := os.WriteFile(coolingStatePath, []byte("0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reader := NewSysPWMFanReader(coolingStatePath, filepath.Join(dir, "hwmon*", "fan1_input"))
	if _, err := reader.Read(); err == nil {
		t.Fatalf("expected an error when no hwmon fan1_input matches")
	}
}
