package oled

import (
	"image"
	"slices"
	"testing"

	"github.com/dnitros/pironman/internal/sysstats"
)

func TestIPRowsLabelsInterfacesAndPagesInGroupsOfThree(t *testing.T) {
	snap := sysstats.Snapshot{Interfaces: map[string]string{
		"wlan0":      "10.0.0.2",
		"eth0":       "192.168.1.5",
		"tailscale0": "100.64.0.1",
		"usb0":       "172.16.0.2",
	}}

	first := []pageRow{{label: "ETH", value: "192.168.1.5"}, {label: "TS", value: "100.64.0.1"}, {label: "USB0", value: "172.16.0.2"}}
	second := []pageRow{{label: "WLAN", value: "10.0.0.2"}}
	for idx, want := range [][]pageRow{first, second, first} {
		if got := ipRows(snap, idx); !slices.Equal(got, want) {
			t.Fatalf("scrollIdx=%d: ipRows = %+v, want %+v", idx, got, want)
		}
	}
}

func TestIPRowsEmptyWithoutInterfaces(t *testing.T) {
	if got := ipRows(sysstats.Snapshot{}, 0); len(got) != 0 {
		t.Fatalf("ipRows = %+v, want none", got)
	}
}

func TestRenderIPsDrawsBoxedLabelAndValuePerRow(t *testing.T) {
	rows := []pageRow{{label: "ETH", value: "192.168.1.5"}, {label: "WLAN", value: "10.0.0.2"}, {label: "TS", value: "100.64.0.1"}}
	img := renderIPs(rows)

	for i := range rows {
		y := i * ipRowPitch
		box := image.Rect(0, y, ipLabelWidth+1, y+ipLabelHeight+1)
		if lit := litIn(img, box); lit == 0 || lit == box.Dx()*box.Dy() {
			t.Fatalf("row %d: label box lit %d of %d pixels, want a lit box with dark text", i, lit, box.Dx()*box.Dy())
		}
		if litIn(img, image.Rect(ipValueX, y, 128, y+ipLabelHeight)) == 0 {
			t.Fatalf("row %d: IP value drew no pixels", i)
		}
	}
}
