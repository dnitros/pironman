package handlers

import (
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/oled"
	"github.com/dnitros/pironman/internal/rgb"
)

func StatusHandler(rgbStore *rgb.Store, oledMachine *oled.Machine) ipc.Handler {
	return func(args map[string]any) (any, error) {
		rgbState := rgbStore.State()
		oledState := oledMachine.State()
		return map[string]any{
			"enabled":    rgbState.Enabled,
			"color":      rgbState.Color,
			"brightness": rgbState.Brightness,
			"oled_awake": oledState.Awake,
			"oled_page":  oledState.Page,
		}, nil
	}
}
