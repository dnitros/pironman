package oled

import (
	"fmt"
	"image"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

const (
	perfRightX     = 66
	rowsPerPage    = 3
	ipRowPitch     = 21
	ipLabelWidth   = 30
	ipValueX       = 35
	diskRowPitch   = 22
	diskValueX     = 34
	diskBarOffset  = 11
	diskBarHeight  = 6
	emptyMessageY  = 18
	displayCenterX = hardware.SSD1306Width / 2
)

type performanceInfo struct {
	cpu, temp, ram, ramUsage, fan string
}

type pageRow struct {
	label, value string
	percent      float64
}

func performanceValues(snap sysstats.Snapshot, fan string) performanceInfo {
	return performanceInfo{
		cpu:      fmt.Sprintf("%.0f%%", snap.CPUPercent),
		temp:     fmt.Sprintf("%.0f°C", snap.CPUTempC),
		ram:      fmt.Sprintf("%.0f%%", snap.MemPercent),
		ramUsage: formatUsedTotal(snap.MemUsedBytes, snap.MemTotalBytes, 1),
		fan:      fan,
	}
}

func fanRPMText(fan hardware.PWMFanReader) string {
	if fan == nil {
		return "--"
	}
	state, err := fan.Read()
	if err != nil {
		return "--"
	}
	return strconv.Itoa(state.SpeedRPM)
}

func ipRows(snap sysstats.Snapshot, scrollIdx int) []pageRow {
	if len(snap.Interfaces) == 0 {
		return nil
	}
	names := slices.Sorted(maps.Keys(snap.Interfaces))
	start, end := paginate(len(names), rowsPerPage, scrollIdx)

	rows := make([]pageRow, 0, end-start)
	for _, name := range names[start:end] {
		rows = append(rows, pageRow{label: interfaceLabel(name), value: snap.Interfaces[name]})
	}
	return rows
}

func interfaceLabel(name string) string {
	switch {
	case strings.HasPrefix(name, "eth"):
		return "ETH"
	case strings.HasPrefix(name, "wlan"):
		return "WLAN"
	case strings.HasPrefix(name, "tailscale"):
		return "TS"
	default:
		return strings.ToUpper(name)
	}
}

func diskRows(snap sysstats.Snapshot, scrollIdx int) []pageRow {
	if len(snap.Disks) == 0 {
		return nil
	}
	start, end := paginate(len(snap.Disks), rowsPerPage, scrollIdx)

	rows := make([]pageRow, 0, end-start)
	for _, d := range snap.Disks[start:end] {
		rows = append(rows, pageRow{
			label:   strings.ToUpper(d.Type),
			value:   formatUsedTotal(d.UsedBytes, d.TotalBytes, 1),
			percent: d.Percent,
		})
	}
	return rows
}

func renderPerformance(v performanceInfo) *image.Gray {
	img := newFrame()
	drawText(img, textFace, "CPU", 0, 0, pixelOn)
	drawText(img, bigFace, v.cpu, 0, 6, pixelOn)
	drawText(img, textFace, "TEMP", perfRightX, 0, pixelOn)
	drawText(img, bigFace, v.temp, perfRightX, 6, pixelOn)
	drawText(img, textFace, "RAM", 0, 27, pixelOn)
	drawText(img, bigFace, v.ram, 0, 33, pixelOn)
	drawText(img, textFace, "FAN RPM", perfRightX, 27, pixelOn)
	drawText(img, bigFace, v.fan, perfRightX, 33, pixelOn)
	drawText(img, textFace, v.ramUsage, 0, 52, pixelOn)
	return img
}

func renderIPs(rows []pageRow) *image.Gray {
	if len(rows) == 0 {
		return renderEmptyMessage("OFFLINE")
	}
	img := newFrame()
	for i, r := range rows {
		y := i * ipRowPitch
		fillRect(img, 0, y, ipLabelWidth, y+mixRowHeight, pixelOn)
		label := fitLine(textFace, r.label, ipLabelWidth-1)
		drawText(img, textFace, label, centeredX(textFace, label, ipLabelWidth/2), y, pixelOff)
		drawText(img, textFace, r.value, ipValueX, y, pixelOn)
	}
	return img
}

func renderDisks(rows []pageRow) *image.Gray {
	if len(rows) == 0 {
		return renderEmptyMessage("NO DISKS")
	}
	img := newFrame()
	for i, r := range rows {
		y := i * diskRowPitch
		drawText(img, textFace, r.label, 0, y, pixelOn)
		drawText(img, textFace, r.value, diskValueX, y, pixelOn)
		drawBar(img, 0, y+diskBarOffset, hardware.SSD1306Width-1, diskBarHeight, r.percent)
	}
	return img
}

func renderEmptyMessage(msg string) *image.Gray {
	img := newFrame()
	drawText(img, bigFace, msg, centeredX(bigFace, msg, displayCenterX), emptyMessageY, pixelOn)
	return img
}
