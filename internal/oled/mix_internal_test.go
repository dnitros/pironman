package oled

import (
	"image"
	"testing"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

func TestComputeMixPageValuesShowsDisconnectedWhenNoInterfaces(t *testing.T) {
	v := computeMixPageValues(sysstats.Snapshot{}, 0)

	if v.ipText != "DISCONNECTED" {
		t.Fatalf("ipText = %q, want %q", v.ipText, "DISCONNECTED")
	}
}

func TestComputeMixPageValuesCyclesInterfacesByScrollIndexInSortedOrder(t *testing.T) {
	snap := sysstats.Snapshot{Interfaces: map[string]string{
		"wlan0": "10.0.0.2",
		"eth0":  "192.168.1.5",
	}}

	if got := computeMixPageValues(snap, 0).ipText; got != "192.168.1.5" {
		t.Fatalf("scrollIdx=0: got %q, want eth0's IP first (sorted order)", got)
	}
	if got := computeMixPageValues(snap, 1).ipText; got != "10.0.0.2" {
		t.Fatalf("scrollIdx=1: got %q, want wlan0's IP second (sorted order)", got)
	}
	if got := computeMixPageValues(snap, 2).ipText; got != "192.168.1.5" {
		t.Fatalf("scrollIdx=2: got %q, want wraparound back to eth0", got)
	}
}

func TestComputeMixPageValuesAggregatesDiskUsageAcrossAllDisks(t *testing.T) {
	snap := sysstats.Snapshot{Disks: []sysstats.Disk{
		{UsedBytes: 10 << 30, TotalBytes: 20 << 30},
		{UsedBytes: 5 << 30, TotalBytes: 20 << 30},
	}}

	v := computeMixPageValues(snap, 0)

	if v.diskUsedBytes != 15<<30 || v.diskTotalBytes != 40<<30 {
		t.Fatalf("disk used/total = %d/%d, want %d/%d", v.diskUsedBytes, v.diskTotalBytes, 15<<30, 40<<30)
	}
	if want := 100 * 15.0 / 40.0; v.diskPercent != want {
		t.Fatalf("diskPercent = %v, want %v", v.diskPercent, want)
	}
}

func TestComputeMixPageValuesZeroDiskPercentWhenNoDisks(t *testing.T) {
	v := computeMixPageValues(sysstats.Snapshot{}, 0)

	if v.diskPercent != 0 {
		t.Fatalf("diskPercent = %v, want 0", v.diskPercent)
	}
}

func TestComputeMixPageValuesCopiesCPUAndMemStats(t *testing.T) {
	snap := sysstats.Snapshot{
		CPUPercent: 42, CPUTempC: 55.5,
		MemUsedBytes: 1 << 30, MemTotalBytes: 4 << 30, MemPercent: 25,
	}

	v := computeMixPageValues(snap, 0)

	if v.cpuPercent != 42 || v.cpuTempC != 55.5 {
		t.Fatalf("cpu = %v/%v, want 42/55.5", v.cpuPercent, v.cpuTempC)
	}
	if v.memUsedBytes != 1<<30 || v.memTotalBytes != 4<<30 || v.memPercent != 25 {
		t.Fatalf("mem = %d/%d/%v, want %d/%d/25", v.memUsedBytes, v.memTotalBytes, v.memPercent, 1<<30, 4<<30)
	}
}

func TestRenderMixPageProducesNonBlankImage(t *testing.T) {
	img := renderMixPage(mixPageValues{ipText: "192.168.1.5", cpuPercent: 42, cpuTempC: 55.5, memPercent: 33, diskPercent: 50})

	var lit int
	for _, p := range img.Pix {
		if p > 0 {
			lit++
		}
	}
	if lit == 0 {
		t.Fatal("renderMixPage produced an entirely blank image")
	}
}

func TestDrawBarFillsProportionallyToPercent(t *testing.T) {
	full := renderBarOnly(100)
	half := renderBarOnly(50)
	empty := renderBarOnly(0)

	if litCount(full) <= litCount(half) {
		t.Fatalf("100%% bar (%d lit px) should have more lit pixels than 50%% bar (%d)", litCount(full), litCount(half))
	}
	if litCount(half) <= litCount(empty) {
		t.Fatalf("50%% bar (%d lit px) should have more lit pixels than 0%% bar (%d)", litCount(half), litCount(empty))
	}
}

func renderBarOnly(percent float64) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, hardware.SSD1306Width, hardware.SSD1306Height))
	drawBar(img, percent, 0, 0, 100, 10)
	return img
}

func litCount(img *image.Gray) int {
	n := 0
	for _, p := range img.Pix {
		if p > 0 {
			n++
		}
	}
	return n
}
