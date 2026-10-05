package oled

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

type mixPageValues struct {
	ipText         string
	cpuPercent     float64
	cpuTempC       float64
	memUsedBytes   uint64
	memTotalBytes  uint64
	memPercent     float64
	diskUsedBytes  uint64
	diskTotalBytes uint64
	diskPercent    float64
}

func computeMixPageValues(snap sysstats.Snapshot, scrollIdx int) mixPageValues {
	ipText := "DISCONNECTED"
	if names := sortedInterfaceNames(snap); len(names) > 0 {
		name := names[scrollIdx%len(names)]
		ipText = snap.Interfaces[name]
	}

	var diskUsed, diskTotal uint64
	for _, d := range snap.Disks {
		diskUsed += d.UsedBytes
		diskTotal += d.TotalBytes
	}
	var diskPercent float64
	if diskTotal > 0 {
		diskPercent = 100 * float64(diskUsed) / float64(diskTotal)
	}

	return mixPageValues{
		ipText:         ipText,
		cpuPercent:     snap.CPUPercent,
		cpuTempC:       snap.CPUTempC,
		memUsedBytes:   snap.MemUsedBytes,
		memTotalBytes:  snap.MemTotalBytes,
		memPercent:     snap.MemPercent,
		diskUsedBytes:  diskUsed,
		diskTotalBytes: diskTotal,
		diskPercent:    diskPercent,
	}
}

func formatGBPair(usedBytes, totalBytes uint64) string {
	return fmt.Sprintf("%.1f/%.1fG", float64(usedBytes)/gigabyte, float64(totalBytes)/gigabyte)
}

const (
	gaugeCenterX = 18
	gaugeRadius  = 15
	cpuGaugeY    = 27
	tempGaugeY   = 48

	rightColX = 39
	rightColW = hardware.SSD1306Width - rightColX
	ipRowY    = 0
	memInfoY  = 17
	memBarY   = 29
	diskInfoY = 41
	diskBarY  = 53
	rowHeight = 10
)

func renderMixPage(v mixPageValues) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, hardware.SSD1306Width, hardware.SSD1306Height))
	lit := &font.Drawer{Dst: img, Src: image.NewUniform(color.Gray{Y: 255}), Face: textFace}
	dark := &font.Drawer{Dst: img, Src: image.NewUniform(color.Gray{Y: 0}), Face: textFace}

	centeredText(lit, "CPU", gaugeCenterX, 0)
	drawPieGauge(img, v.cpuPercent, gaugeCenterX, cpuGaugeY, gaugeRadius, 180, 360)
	centeredText(lit, fmt.Sprintf("%.1f%%", v.cpuPercent), gaugeCenterX, cpuGaugeY)

	centeredText(lit, fmt.Sprintf("%.1fC", v.cpuTempC), gaugeCenterX, tempGaugeY-10)
	drawPieGauge(img, v.cpuTempC, gaugeCenterX, tempGaugeY, gaugeRadius, 0, 180)

	leftText(lit, fmt.Sprintf("RAM: %s", formatGBPair(v.memUsedBytes, v.memTotalBytes)), rightColX, memInfoY)
	drawBar(img, v.memPercent, rightColX, memBarY, rightColW, rowHeight)

	leftText(lit, fmt.Sprintf("DISK: %s", formatGBPair(v.diskUsedBytes, v.diskTotalBytes)), rightColX, diskInfoY)
	drawBar(img, v.diskPercent, rightColX, diskBarY, rightColW, rowHeight)

	fillRect(img, rightColX, ipRowY, rightColX+rightColW, ipRowY+rowHeight, 255)
	centeredText(dark, v.ipText, rightColX+rightColW/2, ipRowY+1)

	return img
}

func centeredText(d *font.Drawer, s string, cx, topY int) {
	width := font.MeasureString(textFace, s)
	d.Dot = fixed.Point26_6{X: fixed.I(cx) - width/2, Y: fixed.I(topY + textBaselineInRow)}
	d.DrawString(s)
}

func leftText(d *font.Drawer, s string, x, topY int) {
	d.Dot = fixed.Point26_6{X: fixed.I(x), Y: fixed.I(topY + textBaselineInRow)}
	d.DrawString(s)
}

func fillRect(img *image.Gray, x0, y0, x1, y1 int, v uint8) {
	draw.Draw(img, image.Rect(x0, y0, x1, y1), image.NewUniform(color.Gray{Y: v}), image.Point{}, draw.Src)
}

func rectOutline(img *image.Gray, x0, y0, x1, y1 int) {
	fillRect(img, x0, y0, x1, y0+1, 255)
	fillRect(img, x0, y1-1, x1, y1, 255)
	fillRect(img, x0, y0, x0+1, y1, 255)
	fillRect(img, x1-1, y0, x1, y1, 255)
}

func drawBar(img *image.Gray, percent float64, x, y, w, h int) {
	rectOutline(img, x, y, x+w, y+h)
	fillRect(img, x, y, x+int(float64(w)*clampPercent(percent)/100), y+h, 255)
}

func drawPieGauge(img *image.Gray, percent float64, cx, cy, r int, startDeg, endDeg float64) {
	valueDeg := startDeg + (endDeg-startDeg)*clampPercent(percent)/100
	strokeArc(img, cx, cy, r, startDeg, endDeg)
	strokeRadius(img, cx, cy, r, startDeg)
	strokeRadius(img, cx, cy, r, endDeg)
	fillSector(img, cx, cy, r, startDeg, valueDeg)
}

func clampPercent(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

func fillSector(img *image.Gray, cx, cy, r int, startDeg, endDeg float64) {
	forEachPixelInRadius(cx, cy, r, func(x, y int, dx, dy float64) {
		if dx*dx+dy*dy <= float64(r*r) && angleInSweep(math.Atan2(dy, dx), startDeg, endDeg) {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	})
}

func strokeArc(img *image.Gray, cx, cy, r int, startDeg, endDeg float64) {
	rf := float64(r)
	forEachPixelInRadius(cx, cy, r, func(x, y int, dx, dy float64) {
		d := math.Hypot(dx, dy)
		if d <= rf && d > rf-1.5 && angleInSweep(math.Atan2(dy, dx), startDeg, endDeg) {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	})
}

func strokeRadius(img *image.Gray, cx, cy, r int, angleDeg float64) {
	rad := angleDeg * math.Pi / 180
	steps := r * 2
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps) * float64(r)
		x := cx + int(math.Round(t*math.Cos(rad)))
		y := cy + int(math.Round(t*math.Sin(rad)))
		if x >= 0 && y >= 0 && x < hardware.SSD1306Width && y < hardware.SSD1306Height {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}
}

func forEachPixelInRadius(cx, cy, r int, fn func(x, y int, dx, dy float64)) {
	for y := cy - r; y <= cy+r; y++ {
		if y < 0 || y >= hardware.SSD1306Height {
			continue
		}
		for x := cx - r; x <= cx+r; x++ {
			if x < 0 || x >= hardware.SSD1306Width {
				continue
			}
			fn(x, y, float64(x-cx), float64(y-cy))
		}
	}
}

func angleInSweep(angleRad float64, startDeg, endDeg float64) bool {
	angle := mod360(angleRad * 180 / math.Pi)
	start := mod360(startDeg)
	end := mod360(endDeg)
	if start <= end {
		return angle >= start && angle <= end
	}
	return angle >= start || angle <= end
}

func mod360(deg float64) float64 {
	deg = math.Mod(deg, 360)
	if deg < 0 {
		deg += 360
	}
	return deg
}
