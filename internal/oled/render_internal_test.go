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
