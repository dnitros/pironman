package oled

import (
	"fmt"
	"image"
	"strconv"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

const perfRightX = 66

type performanceInfo struct {
	cpu, temp, ram, ramUsage, fan string
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
