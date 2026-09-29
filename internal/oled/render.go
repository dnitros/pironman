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

const textLineHeight = 13 // basicfont.Face7x13's line height

// mixLines builds the mix page's four display lines: the interface at
// scrollIdx (cycling through interfaces in sorted name order, "disconnected"
// when there are none), CPU usage, CPU temperature, and RAM usage.
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

// renderLines blits lines onto a fresh SSD1306-sized canvas, one per row.
func renderLines(lines []string) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, hardware.SSD1306Width, hardware.SSD1306Height))
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Gray{Y: 255}),
		Face: basicfont.Face7x13,
	}
	for i, line := range lines {
		d.Dot = fixed.Point26_6{X: fixed.I(0), Y: fixed.I((i + 1) * textLineHeight)}
		d.DrawString(line)
	}
	return img
}
