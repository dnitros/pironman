package oled

import (
	"image"
	"slices"
	"testing"

	"github.com/dnitros/pironman/internal/sysstats"
)

func TestDiskRowsFormatsAndPagesInGroupsOfThree(t *testing.T) {
	disk := func(typ string) sysstats.Disk {
		return sysstats.Disk{Type: typ, UsedBytes: 7475 << 20, TotalBytes: 14 << 30, Percent: 52.1}
	}
	snap := sysstats.Snapshot{Disks: []sysstats.Disk{disk("sd"), disk("nvme"), disk("usb"), disk("raid")}}

	row := func(label string) pageRow { return pageRow{label: label, value: "7.3/14.0 GB", percent: 52.1} }
	first := []pageRow{row("SD"), row("NVME"), row("USB")}
	second := []pageRow{row("RAID")}
	for idx, want := range [][]pageRow{first, second, first} {
		if got := diskRows(snap, idx); !slices.Equal(got, want) {
			t.Fatalf("scrollIdx=%d: diskRows = %+v, want %+v", idx, got, want)
		}
	}
}

func TestRenderDisksBarFillsProportionally(t *testing.T) {
	interior := image.Rect(1, diskBarOffset+1, 127, diskBarOffset+diskBarHeight)
	full := interior.Dx() * interior.Dy()
	for _, c := range []struct{ percent, want float64 }{{0, 0}, {50, 0.5}, {100, 1}} {
		img := renderDisks([]pageRow{{label: "SD", value: "1.0/2.0 GB", percent: c.percent}})
		got := float64(litIn(img, interior)) / float64(full)
		if got < c.want-0.02 || got > c.want+0.02 {
			t.Fatalf("bar at %v%%: interior %.2f lit, want %.2f", c.percent, got, c.want)
		}
	}
}
