package oled

import (
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

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

	maxWidth := fixed.I(hardware.SSD1306Width)
	for _, line := range diskLines(snap, 0) {
		if fitted := fitLine(line, maxWidth); fitted != line {
			t.Fatalf("line %q was truncated to %q, want it to already fit the display width", line, fitted)
		}
		if font.MeasureString(textFace, line) > maxWidth {
			t.Fatalf("line %q measures wider than the %dpx display", line, hardware.SSD1306Width)
		}
	}
}

func TestIpsLinesShowsDisconnectedWhenNoInterfaces(t *testing.T) {
	snap := sysstats.Snapshot{}

	lines := ipsLines(snap, 0)

	if len(lines) != 1 || lines[0] != "disconnected" {
		t.Fatalf("ipsLines = %v, want [\"disconnected\"]", lines)
	}
}

func TestIpsLinesShowsAllInterfacesSortedWhenThreeOrFewer(t *testing.T) {
	snap := sysstats.Snapshot{Interfaces: map[string]string{
		"wlan0": "10.0.0.2",
		"eth0":  "192.168.1.5",
	}}

	lines := ipsLines(snap, 0)

	want := []string{"eth0 192.168.1.5", "wlan0 10.0.0.2"}
	if len(lines) != len(want) || lines[0] != want[0] || lines[1] != want[1] {
		t.Fatalf("ipsLines = %v, want %v", lines, want)
	}
}

func TestIpsLinesWindowsFourOrMoreInterfacesInGroupsOfThreeByScrollIndex(t *testing.T) {
	snap := sysstats.Snapshot{Interfaces: map[string]string{
		"wlan0": "10.0.0.2",
		"eth0":  "192.168.1.5",
		"eth1":  "192.168.1.6",
		"lo":    "127.0.0.1",
	}}

	got0 := ipsLines(snap, 0)
	want0 := []string{"eth0 192.168.1.5", "eth1 192.168.1.6", "lo 127.0.0.1"}
	if len(got0) != len(want0) || got0[0] != want0[0] || got0[1] != want0[1] || got0[2] != want0[2] {
		t.Fatalf("scrollIdx=0: got %v, want %v", got0, want0)
	}

	got1 := ipsLines(snap, 1)
	want1 := []string{"wlan0 10.0.0.2"}
	if len(got1) != len(want1) || got1[0] != want1[0] {
		t.Fatalf("scrollIdx=1: got %v, want %v", got1, want1)
	}

	got2 := ipsLines(snap, 2)
	if len(got2) != len(want0) || got2[0] != want0[0] {
		t.Fatalf("scrollIdx=2: got %v, want wraparound back to %v", got2, want0)
	}
}

func TestFitLineLeavesShortLinesUnchanged(t *testing.T) {
	maxWidth := fixed.I(hardware.SSD1306Width)
	if got := fitLine("CPU 12%", maxWidth); got != "CPU 12%" {
		t.Fatalf("fitLine(%q) = %q, want unchanged", "CPU 12%", got)
	}
}

func TestFitLineTruncatesToFitDisplayWidth(t *testing.T) {
	long := "wlan0 192.168.1.100 on a very long interface description"
	maxWidth := fixed.I(hardware.SSD1306Width)
	if font.MeasureString(textFace, long) <= maxWidth {
		t.Fatalf("test fixture %q must be wider than the display to be meaningful", long)
	}

	got := fitLine(long, maxWidth)

	if font.MeasureString(textFace, got) > maxWidth {
		t.Fatalf("fitLine(%q) = %q, still too wide for the display", long, got)
	}
	if got == long {
		t.Fatalf("expected fitLine to actually truncate %q", long)
	}
}

func TestFitLineTruncatesRuneWiseNotByteWise(t *testing.T) {
	long := "日本語表示テキストが長すぎる場合の切り詰めテスト"
	maxWidth := fixed.I(40)

	got := fitLine(long, maxWidth)

	if !utf8Valid(got) {
		t.Fatalf("fitLine(%q) = %q, not valid UTF-8", long, got)
	}
}

func utf8Valid(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

func TestPerformanceLinesFormatsCPUMemAndTemp(t *testing.T) {
	snap := sysstats.Snapshot{
		CPUPercent:    12.4,
		CPUTempC:      45.67,
		MemPercent:    33.2,
		MemUsedBytes:  1 << 30,
		MemTotalBytes: 4 << 30,
	}

	lines := performanceLines(snap)

	if lines[0] != "CPU 12%" {
		t.Fatalf("lines[0] = %q, want %q", lines[0], "CPU 12%")
	}
	if lines[1] != "RAM 33%" {
		t.Fatalf("lines[1] = %q, want %q", lines[1], "RAM 33%")
	}
	if lines[2] != "1.0/4.0GB" {
		t.Fatalf("lines[2] = %q, want %q", lines[2], "1.0/4.0GB")
	}
	if lines[3] != "45.7C" {
		t.Fatalf("lines[3] = %q, want %q", lines[3], "45.7C")
	}
}

func TestRenderLinesProducesNonBlankImage(t *testing.T) {
	img := renderLines([]string{"eth0 192.168.1.5", "CPU 12%"})

	var lit int
	for _, p := range img.Pix {
		if p > 0 {
			lit++
		}
	}
	if lit == 0 {
		t.Fatal("renderLines produced an entirely blank image")
	}
}
