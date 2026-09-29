package sysstats

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParseCPUStatComputesIdleAndTotal(t *testing.T) {
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
	mountsPath := writeFixture(t, dir, "mounts", "")

	old := listInterfaces
	listInterfaces = func() ([]netIface, error) {
		return []netIface{{Name: "eth0", IP: "192.168.1.5"}}, nil
	}
	t.Cleanup(func() { listInterfaces = old })

	src := NewProcSource(statPath, thermalPath, meminfoPath, mountsPath)

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

	if snap.CPUPercent != 0 {
		t.Fatalf("CPUPercent on first sample = %v, want 0", snap.CPUPercent)
	}
}

func TestProcSourceSnapshotComputesCPUPercentFromDelta(t *testing.T) {
	dir := t.TempDir()
	statPath := filepath.Join(dir, "stat")
	thermalPath := writeFixture(t, dir, "temp", "40000\n")
	meminfoPath := writeFixture(t, dir, "meminfo", "MemTotal:        1000 kB\nMemAvailable:     500 kB\n")
	mountsPath := writeFixture(t, dir, "mounts", "")

	old := listInterfaces
	listInterfaces = func() ([]netIface, error) { return nil, nil }
	t.Cleanup(func() { listInterfaces = old })

	src := NewProcSource(statPath, thermalPath, meminfoPath, mountsPath)

	writeFixture(t, dir, "stat", "cpu  100 0 50 800 20 0 0 0 0 0\n")
	if _, err := src.Snapshot(); err != nil {
		t.Fatalf("first Snapshot: %v", err)
	}

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
	mountsPath := writeFixture(t, dir, "mounts", "")

	old := listInterfaces
	listInterfaces = func() ([]netIface, error) { return nil, nil }
	t.Cleanup(func() { listInterfaces = old })

	src := NewProcSource(statPath, thermalPath, meminfoPath, mountsPath)

	snap, err := src.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap.Interfaces) != 0 {
		t.Fatalf("expected no interfaces, got %v", snap.Interfaces)
	}
}

func TestProcSourceSnapshotReportsDisksFromMounts(t *testing.T) {
	dir := t.TempDir()
	statPath := writeFixture(t, dir, "stat", "cpu  100 0 50 800 20 0 0 0 0 0\n")
	thermalPath := writeFixture(t, dir, "temp", "40000\n")
	meminfoPath := writeFixture(t, dir, "meminfo", "MemTotal:        1000 kB\nMemAvailable:     500 kB\n")
	mountsPath := writeFixture(t, dir, "mounts",
		"/dev/mmcblk0p2 / ext4 rw,noatime 0 0\n"+
			"proc /proc proc rw 0 0\n"+
			"/dev/sda1 /mnt/usb ext4 rw 0 0\n")

	oldListInterfaces := listInterfaces
	listInterfaces = func() ([]netIface, error) { return nil, nil }
	t.Cleanup(func() { listInterfaces = oldListInterfaces })

	oldDiskUsage := diskUsage
	diskUsage = func(mountpoint string) (usedBytes, totalBytes uint64, err error) {
		switch mountpoint {
		case "/":
			return 60, 100, nil
		case "/mnt/usb":
			return 30, 100, nil
		default:
			return 0, 0, fmt.Errorf("unexpected mountpoint %q", mountpoint)
		}
	}
	t.Cleanup(func() { diskUsage = oldDiskUsage })

	src := NewProcSource(statPath, thermalPath, meminfoPath, mountsPath)

	snap, err := src.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	want := []Disk{
		{Type: "sd", UsedBytes: 60, TotalBytes: 100, Percent: 60},
		{Type: "usb", UsedBytes: 30, TotalBytes: 100, Percent: 30},
	}
	if len(snap.Disks) != len(want) {
		t.Fatalf("Disks = %+v, want %+v", snap.Disks, want)
	}
	for i := range want {
		if snap.Disks[i] != want[i] {
			t.Fatalf("Disks[%d] = %+v, want %+v", i, snap.Disks[i], want[i])
		}
	}
}

func TestParseMountsExtractsDeviceAndMountpoint(t *testing.T) {
	data := []byte("/dev/mmcblk0p2 / ext4 rw,noatime 0 0\nproc /proc proc rw 0 0\n\n")

	entries := parseMounts(data)

	want := []mountEntry{
		{device: "/dev/mmcblk0p2", mountpoint: "/"},
		{device: "proc", mountpoint: "/proc"},
	}
	if len(entries) != len(want) {
		t.Fatalf("entries = %+v, want %+v", entries, want)
	}
	for i := range want {
		if entries[i] != want[i] {
			t.Fatalf("entries[%d] = %+v, want %+v", i, entries[i], want[i])
		}
	}
}

func TestClassifyDiskType(t *testing.T) {
	cases := map[string]string{
		"/dev/nvme0n1p1": "nvme",
		"/dev/mmcblk0p2": "sd",
		"/dev/md0":       "raid",
		"/dev/sda1":      "usb",
		"/dev/whatever":  "hd",
	}
	for device, want := range cases {
		if got := classifyDiskType(device); got != want {
			t.Fatalf("classifyDiskType(%q) = %q, want %q", device, got, want)
		}
	}
}

func TestDisksFromMountsSkipsNonDeviceMountsAndFailedUsageLookups(t *testing.T) {
	entries := []mountEntry{
		{device: "/dev/mmcblk0p2", mountpoint: "/"},
		{device: "tmpfs", mountpoint: "/tmp"},
		{device: "/dev/sda1", mountpoint: "/mnt/usb"},
	}
	usage := func(mountpoint string) (usedBytes, totalBytes uint64, err error) {
		if mountpoint == "/mnt/usb" {
			return 0, 0, fmt.Errorf("statfs failed")
		}
		return 40, 100, nil
	}

	disks := disksFromMounts(entries, usage)

	if len(disks) != 1 {
		t.Fatalf("disks = %+v, want exactly one disk", disks)
	}
	if disks[0] != (Disk{Type: "sd", UsedBytes: 40, TotalBytes: 100, Percent: 40}) {
		t.Fatalf("disks[0] = %+v, want the / disk with 40%% used", disks[0])
	}
}

func TestDisksFromMountsReturnsNoneWhenNoRealDisksMounted(t *testing.T) {
	entries := []mountEntry{{device: "tmpfs", mountpoint: "/tmp"}}
	usage := func(mountpoint string) (usedBytes, totalBytes uint64, err error) {
		t.Fatalf("usage lookup should not run for non-device mounts")
		return 0, 0, nil
	}

	if disks := disksFromMounts(entries, usage); len(disks) != 0 {
		t.Fatalf("disks = %+v, want none", disks)
	}
}
