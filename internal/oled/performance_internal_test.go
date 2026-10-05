package oled

import (
	"errors"
	"image"
	"testing"

	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/sysstats"
)

type fakePWMFan struct {
	state hardware.PWMFanState
	err   error
}

func (f fakePWMFan) Read() (hardware.PWMFanState, error) { return f.state, f.err }

func TestPerformanceValuesFormatsStats(t *testing.T) {
	snap := sysstats.Snapshot{
		CPUPercent:    12.4,
		CPUTempC:      45.6,
		MemPercent:    33.2,
		MemUsedBytes:  1 << 30,
		MemTotalBytes: 4 << 30,
	}

	got := performanceValues(snap, "2400")

	want := performanceInfo{cpu: "12%", temp: "46°C", ram: "33%", ramUsage: "1.0/4.0 GB", fan: "2400"}
	if got != want {
		t.Fatalf("performanceValues = %+v, want %+v", got, want)
	}
}

func TestFanRPMTextShowsDashesWhenUnreadable(t *testing.T) {
	cases := []struct {
		name string
		fan  hardware.PWMFanReader
		want string
	}{
		{"no reader", nil, "--"},
		{"read error", fakePWMFan{err: errors.New("no fan1_input")}, "--"},
		{"spinning", fakePWMFan{state: hardware.PWMFanState{SpeedRPM: 2400}}, "2400"},
		{"stopped", fakePWMFan{}, "0"},
	}
	for _, c := range cases {
		if got := fanRPMText(c.fan); got != c.want {
			t.Fatalf("%s: fanRPMText = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRenderPerformanceDrawsEachValue(t *testing.T) {
	base := performanceInfo{cpu: "12%", temp: "46°C", ram: "33%", ramUsage: "1.0/4.0 GB", fan: "2400"}
	areas := map[string]image.Rectangle{
		"cpu":      image.Rect(0, 10, 64, 24),
		"temp":     image.Rect(66, 10, 128, 24),
		"ram":      image.Rect(0, 37, 64, 51),
		"fan":      image.Rect(66, 37, 128, 51),
		"ramUsage": image.Rect(0, 54, 128, 61),
	}
	for name, area := range areas {
		blanked := base
		switch name {
		case "cpu":
			blanked.cpu = ""
		case "temp":
			blanked.temp = ""
		case "ram":
			blanked.ram = ""
		case "fan":
			blanked.fan = ""
		case "ramUsage":
			blanked.ramUsage = ""
		}
		if litIn(renderPerformance(base), area) == 0 || litIn(renderPerformance(blanked), area) != 0 {
			t.Fatalf("%s: want pixels in %v only when the value is set", name, area)
		}
	}
}
