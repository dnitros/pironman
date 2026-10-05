package oled

import (
	"image"
	"maps"
	"slices"
	"strings"

	"github.com/dnitros/pironman/internal/sysstats"
)

const (
	ipRowPitch    = 21
	ipLabelWidth  = 30
	ipLabelHeight = 10
	ipValueX      = 35
)

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

func renderIPs(rows []pageRow) *image.Gray {
	if len(rows) == 0 {
		return renderEmptyMessage("OFFLINE")
	}
	img := newFrame()
	for i, r := range rows {
		y := i * ipRowPitch
		fillRect(img, 0, y, ipLabelWidth, y+ipLabelHeight, pixelOn)
		label := fitLine(textFace, r.label, ipLabelWidth-1)
		drawText(img, textFace, label, centeredX(textFace, label, ipLabelWidth/2), y, pixelOff)
		drawText(img, textFace, r.value, ipValueX, y, pixelOn)
	}
	return img
}
