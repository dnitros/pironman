package oled

import (
	"testing"

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
