package oled

import (
	"fmt"
	"image"
	"image/color"
	"sort"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

const (
	textLineHeight    = 11
	textBaselineInRow = 12
)

func sortedInterfaceNames(snap sysstats.Snapshot) []string {
	names := make([]string, 0, len(snap.Interfaces))
	for name := range snap.Interfaces {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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

	names := sortedInterfaceNames(snap)
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
		Face: textFace,
	}

	maxWidth := fixed.I(hardware.SSD1306Width)
	for i, line := range lines {
		d.Dot = fixed.Point26_6{X: fixed.I(0), Y: fixed.I(i*textLineHeight + textBaselineInRow)}
		d.DrawString(fitLine(line, maxWidth))
	}
	return img
}

func fitLine(s string, maxWidth fixed.Int26_6) string {
	r := []rune(s)
	for len(r) > 0 && font.MeasureString(textFace, string(r)) > maxWidth {
		r = r[:len(r)-1]
	}
	return string(r)
}
