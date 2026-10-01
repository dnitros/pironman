package oled

import (
	"fmt"
	"image"
	"image/color"
	"sort"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

const textLineHeight = 13

func mixLines(snap sysstats.Snapshot, scrollIdx int) []string {
	ipLine := "disconnected"
	if len(snap.Interfaces) > 0 {
		names := make([]string, 0, len(snap.Interfaces))
		for name := range snap.Interfaces {
			names = append(names, name)
		}
		sort.Strings(names)
		name := names[scrollIdx%len(names)]
		ipLine = fmt.Sprintf("%s %s", name, snap.Interfaces[name])
	}

	return []string{
		ipLine,
		fmt.Sprintf("CPU %.0f%%", snap.CPUPercent),
		fmt.Sprintf("%.1fC", snap.CPUTempC),
		fmt.Sprintf("RAM %.0f%%", snap.MemPercent),
	}
}

func diskLines(snap sysstats.Snapshot, scrollIdx int) []string {
	if len(snap.Disks) == 0 {
		return []string{"no disks found"}
	}

	const groupSize = 3
	start, end := paginate(len(snap.Disks), groupSize, scrollIdx)

	lines := make([]string, 0, end-start)
	for _, d := range snap.Disks[start:end] {
		lines = append(lines, fmt.Sprintf("%s %s %.0f%%", d.Type, formatDiskSize(d.UsedBytes, d.TotalBytes), d.Percent))
	}
	return lines
}

func formatDiskSize(usedBytes, totalBytes uint64) string {
	units := [...]string{"B", "K", "M", "G", "T"}
	scaledUsed, scaledTotal := float64(usedBytes), float64(totalBytes)
	unit := 0
	for scaledTotal >= 1000 && unit < len(units)-1 {
		scaledUsed /= 1024
		scaledTotal /= 1024
		unit++
	}
	if unit == 0 || scaledTotal >= 10 {
		return fmt.Sprintf("%.0f/%.0f%s", scaledUsed, scaledTotal, units[unit])
	}
	return fmt.Sprintf("%.1f/%.1f%s", scaledUsed, scaledTotal, units[unit])
}

func paginate(total, size, idx int) (start, end int) {
	groups := (total + size - 1) / size
	start = (idx % groups) * size
	end = min(start+size, total)
	return start, end
}

const ipsPerPage = 3

func ipsLines(snap sysstats.Snapshot, scrollIdx int) []string {
	if len(snap.Interfaces) == 0 {
		return []string{"disconnected"}
	}

	names := make([]string, 0, len(snap.Interfaces))
	for name := range snap.Interfaces {
		names = append(names, name)
	}
	sort.Strings(names)

	start, end := paginate(len(names), ipsPerPage, scrollIdx)

	lines := make([]string, 0, end-start)
	for _, name := range names[start:end] {
		lines = append(lines, fmt.Sprintf("%s %s", name, snap.Interfaces[name]))
	}
	return lines
}

const gigabyte = 1 << 30

func performanceLines(snap sysstats.Snapshot) []string {
	return []string{
		fmt.Sprintf("CPU %.0f%%", snap.CPUPercent),
		fmt.Sprintf("RAM %.0f%%", snap.MemPercent),
		fmt.Sprintf("%.1f/%.1fGB", float64(snap.MemUsedBytes)/gigabyte, float64(snap.MemTotalBytes)/gigabyte),
		fmt.Sprintf("%.1fC", snap.CPUTempC),
	}
}

func renderLines(lines []string) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, hardware.SSD1306Width, hardware.SSD1306Height))
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Gray{Y: 255}),
		Face: basicfont.Face7x13,
	}
	for i, line := range lines {
		d.Dot = fixed.Point26_6{X: fixed.I(0), Y: fixed.I((i + 1) * textLineHeight)}
		d.DrawString(fitLine(line))
	}
	return img
}

func fitLine(s string) string {
	maxWidth := fixed.I(hardware.SSD1306Width)
	for len(s) > 0 && font.MeasureString(basicfont.Face7x13, s) > maxWidth {
		s = s[:len(s)-1]
	}
	return s
}
