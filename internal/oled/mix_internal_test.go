package oled

import (
	"image"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/dnitros/pironman/internal/sysstats"
)

func TestMixValuesPicksIPByInterfacePreferenceAndScrollsToTailscale(t *testing.T) {
	const eth, wlan, ts = "192.168.1.5", "10.0.0.2", "100.64.0.1"
	cases := []struct {
		name   string
		ifaces map[string]string
		want   []string
	}{
		{"eth0 only", map[string]string{"eth0": eth}, []string{eth}},
		{"wlan0 only", map[string]string{"wlan0": wlan}, []string{wlan}},
		{"both prefer eth", map[string]string{"eth0": eth, "wlan0": wlan}, []string{eth}},
		{"any eth interface", map[string]string{"eth2": eth, "wlan0": wlan}, []string{eth}},
		{"any wlan interface", map[string]string{"wlan1": wlan}, []string{wlan}},
		{"first eth in name order", map[string]string{"eth2": "192.168.2.9", "eth1": eth}, []string{eth}},
		{"neither", map[string]string{}, []string{"OFFLINE"}},
		{"other interfaces ignored", map[string]string{"usb0": "172.16.0.2", "enx0123456789ab": "192.168.3.4"}, []string{"OFFLINE"}},
		{"eth and tailscale", map[string]string{"eth0": eth, "wlan0": wlan, "tailscale0": ts}, []string{eth, ts}},
		{"wlan and tailscale", map[string]string{"wlan0": wlan, "tailscale0": ts}, []string{wlan, ts}},
		{"any tailscale interface", map[string]string{"eth1": eth, "tailscale1": ts}, []string{eth, ts}},
		{"tailscale without LAN is offline", map[string]string{"tailscale0": ts}, []string{"OFFLINE"}},
	}
	for _, c := range cases {
		snap := sysstats.Snapshot{Interfaces: c.ifaces}
		for idx := range 2 * len(c.want) {
			if got, want := mixValues(snap, idx).ip, c.want[idx%len(c.want)]; got != want {
				t.Fatalf("%s, scrollIdx=%d: ip = %q, want %q", c.name, idx, got, want)
			}
		}
	}
}

func TestMixValuesPassesThroughCPUAndRAMStats(t *testing.T) {
	snap := sysstats.Snapshot{
		CPUPercent:    12.4,
		CPUTempC:      45.6,
		MemPercent:    7.6,
		MemUsedBytes:  644 << 20,
		MemTotalBytes: 8090 << 20,
	}

	v := mixValues(snap, 0)

	if v.cpuPercent != 12.4 || v.cpuTempC != 45.6 || v.ramPercent != 7.6 {
		t.Fatalf("mixValues = %+v, want CPU/temp/RAM percent passed through", v)
	}
	if v.ramLabel != "RAM:  0.6/7.9 GB" {
		t.Fatalf("ramLabel = %q, want %q", v.ramLabel, "RAM:  0.6/7.9 GB")
	}
}

func TestMixValuesAggregatesAllDisks(t *testing.T) {
	snap := sysstats.Snapshot{Disks: []sysstats.Disk{
		{UsedBytes: 6 << 30, TotalBytes: 10 << 30},
		{UsedBytes: 1331 << 20, TotalBytes: 4 << 30},
	}}

	v := mixValues(snap, 0)

	if v.diskLabel != "DISK: 7.3/14.0 GB" {
		t.Fatalf("diskLabel = %q, want %q", v.diskLabel, "DISK: 7.3/14.0 GB")
	}
	wantPercent := 100 * float64(6<<30+1331<<20) / float64(14<<30)
	if v.diskPercent != wantPercent {
		t.Fatalf("diskPercent = %v, want %v", v.diskPercent, wantPercent)
	}
}

func TestMixValuesShowsNAWhenNoDisks(t *testing.T) {
	v := mixValues(sysstats.Snapshot{}, 0)

	if v.diskLabel != "DISK: NA" || v.diskPercent != 0 {
		t.Fatalf("disk = %q %v, want %q 0", v.diskLabel, v.diskPercent, "DISK: NA")
	}
}

func TestFormatUsedTotalPicksUnitFromTotal(t *testing.T) {
	cases := []struct {
		used, total uint64
		want        string
	}{
		{512, 1000, "512.0/1000.0 B"},
		{300 << 20, 512 << 20, "300.0/512.0 MB"},
		{1 << 40, 2 << 40, "1.0/2.0 TB"},
	}
	for _, c := range cases {
		if got := formatUsedTotal(c.used, c.total, 1); got != c.want {
			t.Fatalf("formatUsedTotal(%d, %d) = %q, want %q", c.used, c.total, got, c.want)
		}
	}
}

func litIn(img *image.Gray, r image.Rectangle) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if img.GrayAt(x, y).Y != 0 {
				n++
			}
		}
	}
	return n
}

var (
	cpuGaugeArea  = image.Rect(3, 12, 34, 27)
	tempGaugeArea = image.Rect(3, 49, 34, 64)
	ramBarArea    = image.Rect(40, 30, 127, 39)
	diskBarArea   = image.Rect(40, 54, 127, 63)
	ipBoxArea     = image.Rect(39, 0, 128, 11)
)

func TestRenderMixGaugesFillProportionally(t *testing.T) {
	for _, area := range []struct {
		name string
		rect image.Rectangle
		set  func(v *mixInfo, p float64)
	}{
		{"cpu", cpuGaugeArea, func(v *mixInfo, p float64) { v.cpuPercent = p }},
		{"temp", tempGaugeArea, func(v *mixInfo, p float64) { v.cpuTempC = p }},
	} {
		lit := map[float64]int{}
		for _, p := range []float64{0, 50, 100, 150} {
			v := mixInfo{}
			area.set(&v, p)
			lit[p] = litIn(renderMix(v), area.rect)
		}
		if lit[0] == 0 {
			t.Fatalf("%s gauge: empty gauge drew no outline", area.name)
		}
		if !(lit[0] < lit[50] && lit[50] < lit[100]) {
			t.Fatalf("%s gauge: lit pixels at 0/50/100%% = %d/%d/%d, want strictly increasing", area.name, lit[0], lit[50], lit[100])
		}
		half := float64(lit[50]-lit[0]) / float64(lit[100]-lit[0])
		if half < 0.4 || half > 0.6 {
			t.Fatalf("%s gauge: 50%% filled %.2f of the gauge, want about half", area.name, half)
		}
		if lit[150] != lit[100] {
			t.Fatalf("%s gauge: 150%% lit %d pixels, want clamped to the 100%% count %d", area.name, lit[150], lit[100])
		}
	}
}

func TestRenderMixBarsFillProportionally(t *testing.T) {
	for _, area := range []struct {
		name string
		rect image.Rectangle
		set  func(v *mixInfo, p float64)
	}{
		{"ram", ramBarArea, func(v *mixInfo, p float64) { v.ramPercent = p }},
		{"disk", diskBarArea, func(v *mixInfo, p float64) { v.diskPercent = p }},
	} {
		full := area.rect.Dx() * area.rect.Dy()
		for _, c := range []struct{ percent, wantFraction float64 }{{0, 0}, {50, 0.5}, {100, 1}} {
			v := mixInfo{}
			area.set(&v, c.percent)
			got := float64(litIn(renderMix(v), area.rect)) / float64(full)
			if got < c.wantFraction-0.02 || got > c.wantFraction+0.02 {
				t.Fatalf("%s bar at %v%%: interior %.2f lit, want %.2f", area.name, c.percent, got, c.wantFraction)
			}
		}
	}
}

func TestRenderMixDrawsIPAsDarkTextInLitBox(t *testing.T) {
	box := ipBoxArea.Dx() * ipBoxArea.Dy()

	blank := litIn(renderMix(mixInfo{}), ipBoxArea)
	withIP := litIn(renderMix(mixInfo{ip: "192.168.1.34"}), ipBoxArea)

	if blank != box {
		t.Fatalf("empty IP box lit %d of %d pixels, want fully lit", blank, box)
	}
	if withIP >= blank {
		t.Fatalf("IP text cut no dark pixels out of the lit box (%d lit of %d)", withIP, box)
	}
}

func TestRenderMixShowsCPUAndRAMText(t *testing.T) {
	textArea := image.Rect(0, 0, 36, 12)
	if litIn(renderMix(mixInfo{}), textArea) == 0 {
		t.Fatalf("CPU heading drew no pixels")
	}

	labelArea := image.Rect(39, 17, 128, 28)
	if litIn(renderMix(mixInfo{ramLabel: "RAM:  0.6/7.9 GB"}), labelArea) == 0 {
		t.Fatalf("RAM label drew no pixels")
	}
}

func TestMixValuesDropsDecimalsWhenDiskLabelWouldOverflow(t *testing.T) {
	snap := sysstats.Snapshot{Disks: []sysstats.Disk{{UsedBytes: 400 << 30, TotalBytes: 931 << 30}}}

	v := mixValues(snap, 0)

	if v.diskLabel != "DISK: 400/931 GB" {
		t.Fatalf("diskLabel = %q, want %q", v.diskLabel, "DISK: 400/931 GB")
	}
	if w := font.MeasureString(textFace, v.diskLabel); w > fixed.I(mixRightWidth) {
		t.Fatalf("diskLabel %q is %v wide, want it to fit %dpx", v.diskLabel, w, mixRightWidth)
	}
}
