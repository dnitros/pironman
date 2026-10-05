package oled

import (
	"image"
	"strings"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

const (
	diskRowPitch  = 22
	diskValueX    = 34
	diskBarOffset = 11
	diskBarHeight = 6
)

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
