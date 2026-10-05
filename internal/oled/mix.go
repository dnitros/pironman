package oled

import (
	"cmp"
	"fmt"
	"image"
	"maps"
	"math"
	"slices"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/dnitros/pironman/internal/sysstats"
)

const (
	mixLeftCenterX = 18
	mixGaugeRadius = 15
	mixRightX      = 39
	mixRightWidth  = 88
	mixRowHeight   = 10
)

type mixInfo struct {
	ip                      string
	cpuPercent, cpuTempC    float64
	ramLabel, diskLabel     string
	ramPercent, diskPercent float64
}

func mixValues(snap sysstats.Snapshot, scrollIdx int) mixInfo {
	v := mixInfo{
		cpuPercent: snap.CPUPercent,
		cpuTempC:   snap.CPUTempC,
		ramLabel:   usageLabel("RAM:  ", snap.MemUsedBytes, snap.MemTotalBytes),
		ramPercent: snap.MemPercent,
		diskLabel:  "DISK: NA",
	}
	ips := mixIPs(snap.Interfaces)
	v.ip = ips[scrollIdx%len(ips)]

	var used, total uint64
	for _, d := range snap.Disks {
		used += d.UsedBytes
		total += d.TotalBytes
	}
	if total > 0 {
		v.diskLabel = usageLabel("DISK: ", used, total)
		v.diskPercent = 100 * float64(used) / float64(total)
	}
	return v
}

func mixIPs(ifaces map[string]string) []string {
	found := firstIPWithPrefix(ifaces, "eth", "wlan", "tailscale")
	lan := cmp.Or(found["eth"], found["wlan"])
	if lan == "" {
		return []string{"OFFLINE"}
	}
	if ts := found["tailscale"]; ts != "" {
		return []string{lan, ts}
	}
	return []string{lan}
}

func firstIPWithPrefix(ifaces map[string]string, prefixes ...string) map[string]string {
	found := make(map[string]string, len(prefixes))
	for _, name := range slices.Sorted(maps.Keys(ifaces)) {
		for _, prefix := range prefixes {
			if _, ok := found[prefix]; !ok && strings.HasPrefix(name, prefix) {
				found[prefix] = ifaces[name]
			}
		}
	}
	return found
}

func usageLabel(prefix string, used, total uint64) string {
	s := prefix + formatUsedTotal(used, total, 1)
	if font.MeasureString(textFace, s) > fixed.I(mixRightWidth) {
		s = prefix + formatUsedTotal(used, total, 0)
	}
	return s
}

func renderMix(v mixInfo) *image.Gray {
	img := newFrame()

	drawText(img, textFace, "CPU", centeredX(textFace, "CPU", mixLeftCenterX), 0, pixelOn)
	drawGauge(img, mixLeftCenterX, 27, 180, v.cpuPercent)
	cpu := fmt.Sprintf("%.1f%%", v.cpuPercent)
	drawText(img, textFace, cpu, centeredX(textFace, cpu, mixLeftCenterX), 27, pixelOn)
	temp := fmt.Sprintf("%.1f°C", v.cpuTempC)
	drawText(img, textFace, temp, centeredX(textFace, temp, mixLeftCenterX), 37, pixelOn)
	drawGauge(img, mixLeftCenterX, 48, 0, v.cpuTempC)

	drawText(img, textFace, v.ramLabel, mixRightX, 17, pixelOn)
	drawBar(img, mixRightX, 29, mixRightWidth, mixRowHeight, v.ramPercent)
	drawText(img, textFace, v.diskLabel, mixRightX, 41, pixelOn)
	drawBar(img, mixRightX, 53, mixRightWidth, mixRowHeight, v.diskPercent)

	fillRect(img, mixRightX, 0, mixRightX+mixRightWidth, mixRowHeight, pixelOn)
	drawText(img, textFace, v.ip, centeredX(textFace, v.ip, mixRightX+mixRightWidth/2), 0, pixelOff)
	return img
}

func drawGauge(img *image.Gray, cx, cy int, startDeg, percent float64) {
	r := mixGaugeRadius
	sweep := func(dx, dy int) float64 {
		deg := math.Atan2(float64(dy), float64(dx)) * 180 / math.Pi
		return math.Mod(deg-startDeg+720, 360)
	}
	inHalf := func(dx, dy int) bool {
		return dx*dx+dy*dy <= r*r+r && sweep(dx, dy) <= 180
	}
	fillTo := 180 * min(max(percent, 0), 100) / 100

	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if !inHalf(dx, dy) {
				continue
			}
			edge := !inHalf(dx+1, dy) || !inHalf(dx-1, dy) || !inHalf(dx, dy+1) || !inHalf(dx, dy-1)
			if edge || (percent > 0 && sweep(dx, dy) <= fillTo) {
				img.SetGray(cx+dx, cy+dy, pixelOn)
			}
		}
	}
}
