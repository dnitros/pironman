package oled

import (
	"bytes"
	"errors"
	"image"
	"slices"
	"testing"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

type fakePWMFan struct {
	state hardware.PWMFanState
	err   error
}

func (f fakePWMFan) Read() (hardware.PWMFanState, error) { return f.state, f.err }

func TestPerformanceValuesFormatsStats(t *testing.T) {
	snap := sysstats.Snapshot{
		CPUPercent:    12.4,
		CPUTempC:      45.6,
		MemPercent:    33.2,
		MemUsedBytes:  1 << 30,
		MemTotalBytes: 4 << 30,
	}

	got := performanceValues(snap, "2400")

	want := performanceInfo{cpu: "12%", temp: "46°C", ram: "33%", ramUsage: "1.0/4.0 GB", fan: "2400"}
	if got != want {
		t.Fatalf("performanceValues = %+v, want %+v", got, want)
	}
}

func TestFanRPMTextShowsDashesWhenUnreadable(t *testing.T) {
	cases := []struct {
		name string
		fan  hardware.PWMFanReader
		want string
	}{
		{"no reader", nil, "--"},
		{"read error", fakePWMFan{err: errors.New("no fan1_input")}, "--"},
		{"spinning", fakePWMFan{state: hardware.PWMFanState{SpeedRPM: 2400}}, "2400"},
		{"stopped", fakePWMFan{}, "0"},
	}
	for _, c := range cases {
		if got := fanRPMText(c.fan); got != c.want {
			t.Fatalf("%s: fanRPMText = %q, want %q", c.name, got, c.want)
		}
	}
}

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

func TestRenderPerformanceDrawsEachValue(t *testing.T) {
	base := performanceInfo{cpu: "12%", temp: "46°C", ram: "33%", ramUsage: "1.0/4.0 GB", fan: "2400"}
	areas := map[string]image.Rectangle{
		"cpu":      image.Rect(0, 10, 64, 24),
		"temp":     image.Rect(66, 10, 128, 24),
		"ram":      image.Rect(0, 37, 64, 51),
		"fan":      image.Rect(66, 37, 128, 51),
		"ramUsage": image.Rect(0, 54, 128, 61),
	}
	for name, area := range areas {
		blanked := base
		switch name {
		case "cpu":
			blanked.cpu = ""
		case "temp":
			blanked.temp = ""
		case "ram":
			blanked.ram = ""
		case "fan":
			blanked.fan = ""
		case "ramUsage":
			blanked.ramUsage = ""
		}
		if litIn(renderPerformance(base), area) == 0 || litIn(renderPerformance(blanked), area) != 0 {
			t.Fatalf("%s: want pixels in %v only when the value is set", name, area)
		}
	}
}

func TestRenderIPsDrawsBoxedLabelAndValuePerRow(t *testing.T) {
	rows := []pageRow{{label: "ETH", value: "192.168.1.5"}, {label: "WLAN", value: "10.0.0.2"}, {label: "TS", value: "100.64.0.1"}}
	img := renderIPs(rows)

	for i := range rows {
		y := i * ipRowPitch
		box := image.Rect(0, y, ipLabelWidth+1, y+mixRowHeight+1)
		if lit := litIn(img, box); lit == 0 || lit == box.Dx()*box.Dy() {
			t.Fatalf("row %d: label box lit %d of %d pixels, want a lit box with dark text", i, lit, box.Dx()*box.Dy())
		}
		if litIn(img, image.Rect(ipValueX, y, 128, y+mixRowHeight)) == 0 {
			t.Fatalf("row %d: IP value drew no pixels", i)
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

func TestEmptyListPagesShowDistinctMessages(t *testing.T) {
	offline, noDisks := renderIPs(nil), renderDisks(nil)

	if litIn(offline, offline.Bounds()) == 0 || litIn(noDisks, noDisks.Bounds()) == 0 {
		t.Fatalf("empty ips/disk pages drew nothing, want OFFLINE / NO DISKS")
	}
	if bytes.Equal(offline.Pix, noDisks.Pix) {
		t.Fatalf("empty ips and disk pages render identically, want different messages")
	}
}
