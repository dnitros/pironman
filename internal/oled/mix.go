package oled

import (
	"cmp"
	"fmt"
	"image"
	"image/color"
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

var (
	pixelOn  = color.Gray{Y: 255}
	pixelOff = color.Gray{Y: 0}
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
	lan := cmp.Or(firstIPWithPrefix(ifaces, "eth"), firstIPWithPrefix(ifaces, "wlan"))
	if lan == "" {
		return []string{"OFFLINE"}
	}
	if ts := firstIPWithPrefix(ifaces, "tailscale"); ts != "" {
		return []string{lan, ts}
	}
	return []string{lan}
}

func firstIPWithPrefix(ifaces map[string]string, prefix string) string {
	for _, name := range slices.Sorted(maps.Keys(ifaces)) {
		if strings.HasPrefix(name, prefix) {
			return ifaces[name]
		}
	}
	return ""
}

func usageLabel(prefix string, used, total uint64) string {
	s := prefix + formatUsedTotal(used, total, 1)
	if font.MeasureString(textFace, s) > fixed.I(mixRightWidth) {
		s = prefix + formatUsedTotal(used, total, 0)
	}
	return s
}

func formatUsedTotal(used, total uint64, decimals int) string {
	units := [...]string{"B", "KB", "MB", "GB", "TB"}
	u, t := float64(used), float64(total)
	unit := 0
	for t >= 1024 && unit < len(units)-1 {
		u /= 1024
		t /= 1024
		unit++
	}
	return fmt.Sprintf("%.*f/%.*f %s", decimals, u, decimals, t, units[unit])
}

func renderMix(v mixInfo) *image.Gray {
	img := newFrame()

	drawText(img, "CPU", centeredX("CPU", mixLeftCenterX), 0, pixelOn)
	drawGauge(img, mixLeftCenterX, 27, 180, v.cpuPercent)
	cpu := fmt.Sprintf("%.1f%%", v.cpuPercent)
	drawText(img, cpu, centeredX(cpu, mixLeftCenterX), 27, pixelOn)
	temp := fmt.Sprintf("%.1f°C", v.cpuTempC)
	drawText(img, temp, centeredX(temp, mixLeftCenterX), 37, pixelOn)
	drawGauge(img, mixLeftCenterX, 48, 0, v.cpuTempC)

	drawText(img, v.ramLabel, mixRightX, 17, pixelOn)
	drawBar(img, mixRightX, 29, v.ramPercent)
	drawText(img, v.diskLabel, mixRightX, 41, pixelOn)
	drawBar(img, mixRightX, 53, v.diskPercent)

	fillRect(img, mixRightX, 0, mixRightX+mixRightWidth, mixRowHeight, pixelOn)
	drawText(img, v.ip, centeredX(v.ip, mixRightX+mixRightWidth/2), 0, pixelOff)
	return img
}

func drawText(img *image.Gray, s string, x, y int, c color.Gray) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: textFace}
	d.Dot.X = fixed.I(x)
	d.Dot.Y = fixed.I(y) + textFace.Metrics().Ascent
	d.DrawString(s)
}

func centeredX(s string, cx int) int {
	return cx - font.MeasureString(textFace, s).Round()/2
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

func drawBar(img *image.Gray, x, y int, percent float64) {
	x1, y1 := x+mixRightWidth, y+mixRowHeight
	fillRect(img, x, y, x1, y1, pixelOn)
	fillRect(img, x+1, y+1, x1-1, y1-1, pixelOff)
	fillRect(img, x, y, x+int(mixRightWidth*min(max(percent, 0), 100)/100), y1, pixelOn)
}

func fillRect(img *image.Gray, x0, y0, x1, y1 int, c color.Gray) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			img.SetGray(x, y, c)
		}
	}
}
