package oled

import (
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

func TestMixLinesShowsDisconnectedWhenNoInterfaces(t *testing.T) {
	snap := sysstats.Snapshot{CPUPercent: 12, CPUTempC: 45.6, MemPercent: 33}

	lines := mixLines(snap, 0)

	if lines[0] != "disconnected" {
		t.Fatalf("lines[0] = %q, want %q", lines[0], "disconnected")
	}
}

func TestMixLinesShowsInterfaceNameAndIP(t *testing.T) {
	snap := sysstats.Snapshot{Interfaces: map[string]string{"eth0": "192.168.1.5"}}

	lines := mixLines(snap, 0)

	if lines[0] != "eth0 192.168.1.5" {
		t.Fatalf("lines[0] = %q, want %q", lines[0], "eth0 192.168.1.5")
	}
}

func TestMixLinesCyclesInterfacesByScrollIndexInSortedOrder(t *testing.T) {
	snap := sysstats.Snapshot{Interfaces: map[string]string{
		"wlan0": "10.0.0.2",
		"eth0":  "192.168.1.5",
	}}

	if got := mixLines(snap, 0)[0]; got != "eth0 192.168.1.5" {
		t.Fatalf("scrollIdx=0: got %q, want eth0 first (sorted order)", got)
	}
	if got := mixLines(snap, 1)[0]; got != "wlan0 10.0.0.2" {
		t.Fatalf("scrollIdx=1: got %q, want wlan0 second (sorted order)", got)
	}
	if got := mixLines(snap, 2)[0]; got != "eth0 192.168.1.5" {
		t.Fatalf("scrollIdx=2: got %q, want wraparound back to eth0", got)
	}
}

func TestDiskLinesShowsDetectionErrorWhenNoDisksFound(t *testing.T) {
	lines := diskLines(sysstats.Snapshot{}, 0)

	if len(lines) != 1 || lines[0] != "no disks found" {
		t.Fatalf("lines = %+v, want a single detection-error line", lines)
	}
}

func TestDiskLinesShowsAllDisksWhenThreeOrFewer(t *testing.T) {
	snap := sysstats.Snapshot{Disks: []sysstats.Disk{
		{Type: "sd", UsedBytes: 12 << 30, TotalBytes: 32 << 30, Percent: 37.5},
		{Type: "usb", UsedBytes: 1 << 30, TotalBytes: 2 << 30, Percent: 50},
	}}

	lines := diskLines(snap, 0)

	want := []string{"sd 12/32G 38%", "usb 1.0/2.0G 50%"}
	if len(lines) != len(want) {
		t.Fatalf("lines = %+v, want %+v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("lines[%d] = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestDiskLinesWindowsIntoGroupsOfThreeAndScrolls(t *testing.T) {
	disk := func(t string) sysstats.Disk { return sysstats.Disk{Type: t, TotalBytes: 1} }
	snap := sysstats.Snapshot{Disks: []sysstats.Disk{
		disk("a"), disk("b"), disk("c"), disk("d"),
	}}

	first := diskLines(snap, 0)
	if len(first) != 3 || first[0][:1] != "a" || first[2][:1] != "c" {
		t.Fatalf("first group = %+v, want disks a-c", first)
	}

	second := diskLines(snap, 1)
	if len(second) != 1 || second[0][:1] != "d" {
		t.Fatalf("second group = %+v, want just disk d", second)
	}

	wrapped := diskLines(snap, 2)
	if len(wrapped) != 3 || wrapped[0][:1] != "a" {
		t.Fatalf("scrollIdx=2 = %+v, want wraparound back to the first group", wrapped)
	}
}

func TestFormatDiskSizeScalesUnitForSubGiBDisk(t *testing.T) {
	if got := formatDiskSize(502<<20, 512<<20); got != "502/512M" {
		t.Fatalf("formatDiskSize = %q, want %q", got, "502/512M")
	}
}

func TestFormatDiskSizeScalesUnitForMultiTeraByteDisk(t *testing.T) {
	if got := formatDiskSize(2_000_000_000_000, 2_000_000_000_000); got != "1.8/1.8T" {
		t.Fatalf("formatDiskSize = %q, want %q", got, "1.8/1.8T")
	}
}

func TestDiskLinesFitDisplayWidthForExtremeSizes(t *testing.T) {
	snap := sysstats.Snapshot{Disks: []sysstats.Disk{
		{Type: "sd", UsedBytes: 502 << 20, TotalBytes: 512 << 20, Percent: 98},
		{Type: "nvme", UsedBytes: 2_000_000_000_000, TotalBytes: 2_000_000_000_000, Percent: 100},
	}}

	for _, line := range diskLines(snap, 0) {
		if fitted := fitLine(line); fitted != line {
			t.Fatalf("line %q was truncated to %q, want it to already fit the display width", line, fitted)
		}
		if font.MeasureString(basicfont.Face7x13, line) > fixed.I(hardware.SSD1306Width) {
			t.Fatalf("line %q measures wider than the %dpx display", line, hardware.SSD1306Width)
		}
	}
}

func TestFitLineLeavesShortLinesUnchanged(t *testing.T) {
	if got := fitLine("CPU 12%"); got != "CPU 12%" {
		t.Fatalf("fitLine(%q) = %q, want unchanged", "CPU 12%", got)
	}
}

func TestFitLineTruncatesToFitDisplayWidth(t *testing.T) {
	long := "wlan0 192.168.1.100"
	if font.MeasureString(basicfont.Face7x13, long) <= fixed.I(hardware.SSD1306Width) {
		t.Fatalf("test fixture %q must be wider than the display to be meaningful", long)
	}

	got := fitLine(long)

	if font.MeasureString(basicfont.Face7x13, got) > fixed.I(hardware.SSD1306Width) {
		t.Fatalf("fitLine(%q) = %q, still too wide for the display", long, got)
	}
	if got == long {
		t.Fatalf("expected fitLine to actually truncate %q", long)
	}
}

func TestMixLinesFormatsCPUTempAndMem(t *testing.T) {
	snap := sysstats.Snapshot{CPUPercent: 12.4, CPUTempC: 45.67, MemPercent: 33.2}

	lines := mixLines(snap, 0)

	if lines[1] != "CPU 12%" {
		t.Fatalf("lines[1] = %q, want %q", lines[1], "CPU 12%")
	}
	if lines[2] != "45.7C" {
		t.Fatalf("lines[2] = %q, want %q", lines[2], "45.7C")
	}
	if lines[3] != "RAM 33%" {
		t.Fatalf("lines[3] = %q, want %q", lines[3], "RAM 33%")
	}
}
