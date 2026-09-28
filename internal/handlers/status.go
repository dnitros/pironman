package handlers

import (
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/rgb"
)

func StatusHandler(rgbStore *rgb.Store) ipc.Handler {
	return func(args map[string]any) (any, error) {
		state := rgbStore.State()
		return map[string]any{
			"enabled":    state.Enabled,
			"color":      state.Color,
			"brightness": state.Brightness,
		}, nil
	}
}
