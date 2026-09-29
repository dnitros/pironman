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
	groups := (len(snap.Disks) + groupSize - 1) / groupSize
	start := (scrollIdx % groups) * groupSize
	end := start + groupSize
	if end > len(snap.Disks) {
		end = len(snap.Disks)
	}

	lines := make([]string, 0, end-start)
	for _, d := range snap.Disks[start:end] {
		lines = append(lines, fmt.Sprintf("%s %.0fG/%.0fG %.0f%%", d.Type, gib(d.UsedBytes), gib(d.TotalBytes), d.Percent))
	}
	return lines
}

func gib(bytes uint64) float64 {
	return float64(bytes) / (1 << 30)
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
