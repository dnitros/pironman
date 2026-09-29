package sysstats

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCPUStatComputesIdleAndTotal(t *testing.T) {
	// user nice system idle iowait irq softirq steal guest guest_nice
	data := []byte("cpu  100 0 50 800 20 0 0 0 0 0\nignored line\n")

	idle, total, err := parseCPUStat(data)
	if err != nil {
		t.Fatalf("parseCPUStat: %v", err)
	}
	if total != 970 {
		t.Fatalf("total = %d, want 970", total)
	}
	if idle != 820 {
		t.Fatalf("idle = %d, want 820 (idle+iowait)", idle)
	}
}

func TestParseCPUStatRejectsMissingCPULine(t *testing.T) {
	if _, _, err := parseCPUStat([]byte("nothing here\n")); err == nil {
		t.Fatalf("expected an error when no \"cpu \" line is present")
	}
}

func TestParseMemInfoComputesUsedAndTotal(t *testing.T) {
	data := []byte("MemTotal:        1000 kB\nMemFree:          100 kB\nMemAvailable:     400 kB\n")

	used, total, err := parseMemInfo(data)
	if err != nil {
		t.Fatalf("parseMemInfo: %v", err)
	}
	if total != 1000*1024 {
		t.Fatalf("total = %d, want %d", total, 1000*1024)
	}
	if used != 600*1024 {
		t.Fatalf("used = %d, want %d", used, 600*1024)
	}
}

func TestParseMemInfoRejectsMissingFields(t *testing.T) {
	if _, _, err := parseMemInfo([]byte("MemTotal: 1000 kB\n")); err == nil {
		t.Fatalf("expected an error when MemAvailable is missing")
	}
}

func TestParseThermalTempCConvertsMilliCelsius(t *testing.T) {
	got, err := parseThermalTempC([]byte("45678\n"))
	if err != nil {
		t.Fatalf("parseThermalTempC: %v", err)
	}
	if got != 45.678 {
		t.Fatalf("got %v, want 45.678", got)
	}
}

func TestParseThermalTempCRejectsMalformedInput(t *testing.T) {
	if _, err := parseThermalTempC([]byte("not-a-number\n")); err == nil {
		t.Fatalf("expected an error for malformed thermal input")
	}
}

func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
	return path
}

func TestProcSourceSnapshotReportsMemAndTempAndInterfaces(t *testing.T) {
	dir := t.TempDir()
	statPath := writeFixture(t, dir, "stat", "cpu  100 0 50 800 20 0 0 0 0 0\n")
	thermalPath := writeFixture(t, dir, "temp", "45678\n")
	meminfoPath := writeFixture(t, dir, "meminfo", "MemTotal:        1000 kB\nMemAvailable:     400 kB\n")

	old := listInterfaces
	listInterfaces = func() ([]netIface, error) {
		return []netIface{{Name: "eth0", IP: "192.168.1.5"}}, nil
	}
	t.Cleanup(func() { listInterfaces = old })

	src := NewProcSource(statPath, thermalPath, meminfoPath)

	snap, err := src.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.CPUTempC != 45.678 {
		t.Fatalf("CPUTempC = %v, want 45.678", snap.CPUTempC)
	}
	if snap.MemTotalBytes != 1000*1024 {
		t.Fatalf("MemTotalBytes = %d, want %d", snap.MemTotalBytes, 1000*1024)
	}
	if snap.MemUsedBytes != 600*1024 {
		t.Fatalf("MemUsedBytes = %d, want %d", snap.MemUsedBytes, 600*1024)
	}
	if snap.MemPercent != 60 {
		t.Fatalf("MemPercent = %v, want 60", snap.MemPercent)
	}
	if snap.Interfaces["eth0"] != "192.168.1.5" {
		t.Fatalf("Interfaces[eth0] = %q, want 192.168.1.5", snap.Interfaces["eth0"])
	}
	// First sample has no prior CPU reading to diff against.
	if snap.CPUPercent != 0 {
		t.Fatalf("CPUPercent on first sample = %v, want 0", snap.CPUPercent)
	}
}

func TestProcSourceSnapshotComputesCPUPercentFromDelta(t *testing.T) {
	dir := t.TempDir()
	statPath := filepath.Join(dir, "stat")
	thermalPath := writeFixture(t, dir, "temp", "40000\n")
	meminfoPath := writeFixture(t, dir, "meminfo", "MemTotal:        1000 kB\nMemAvailable:     500 kB\n")

	old := listInterfaces
	listInterfaces = func() ([]netIface, error) { return nil, nil }
	t.Cleanup(func() { listInterfaces = old })

	src := NewProcSource(statPath, thermalPath, meminfoPath)

	writeFixture(t, dir, "stat", "cpu  100 0 50 800 20 0 0 0 0 0\n")
	if _, err := src.Snapshot(); err != nil {
		t.Fatalf("first Snapshot: %v", err)
	}

	// total advances by 100 (all in the non-idle "user" bucket), idle unchanged.
	writeFixture(t, dir, "stat", "cpu  200 0 50 800 20 0 0 0 0 0\n")
	snap, err := src.Snapshot()
	if err != nil {
		t.Fatalf("second Snapshot: %v", err)
	}
	if snap.CPUPercent != 100 {
		t.Fatalf("CPUPercent = %v, want 100", snap.CPUPercent)
	}
}

func TestProcSourceSnapshotReportsEmptyInterfacesWhenDisconnected(t *testing.T) {
	dir := t.TempDir()
	statPath := writeFixture(t, dir, "stat", "cpu  100 0 50 800 20 0 0 0 0 0\n")
	thermalPath := writeFixture(t, dir, "temp", "40000\n")
	meminfoPath := writeFixture(t, dir, "meminfo", "MemTotal:        1000 kB\nMemAvailable:     500 kB\n")

	old := listInterfaces
	listInterfaces = func() ([]netIface, error) { return nil, nil }
	t.Cleanup(func() { listInterfaces = old })

	src := NewProcSource(statPath, thermalPath, meminfoPath)

	snap, err := src.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap.Interfaces) != 0 {
		t.Fatalf("expected no interfaces, got %v", snap.Interfaces)
	}
}
