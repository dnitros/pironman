package handlers

import (
	"fmt"

	"github.com/dnitros/pironman/internal/fan"
	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/oled"
	"github.com/dnitros/pironman/internal/rgb"
)

func StatusHandler(rgbStore *rgb.Store, oledMachine *oled.Machine, fanMachine *fan.Machine, pwmFanReader hardware.PWMFanReader) ipc.Handler {
	return func(args map[string]any) (any, error) {
		state := rgbStore.State()
		oledState := oledMachine.State()
		fanState := fanMachine.State()
		pwmFanState, err := pwmFanReader.Read()
		if err != nil {
			return nil, fmt.Errorf("read PWM fan state: %w", err)
		}
		return map[string]any{
			"enabled":           state.Enabled,
			"color":             state.Color,
			"brightness":        state.Brightness,
			"oled_awake":        oledState.Awake,
			"oled_page":         oledState.Page,
			"case_fan_mode":     fanState.Mode,
			"case_fan_relay_on": fanState.RelayOn,
			"pwm_fan_level":     pwmFanState.Level,
			"pwm_fan_speed_rpm": pwmFanState.SpeedRPM,
		}, nil
	}
}
