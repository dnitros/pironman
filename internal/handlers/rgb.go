package handlers

import (
	"fmt"
	"math"
	"sync"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/rgb"
)

func RGBHandlers(store *rgb.Store, cfg *config.Config, cfgPath string, cfgMu *sync.Mutex) map[string]ipc.Handler {
	return map[string]ipc.Handler{
		"rgb.on":         rgbSetHandler(store, cfg, cfgPath, cfgMu, true),
		"rgb.off":        rgbSetHandler(store, cfg, cfgPath, cfgMu, false),
		"rgb.color":      rgbColorHandler(store, cfg, cfgPath, cfgMu),
		"rgb.brightness": rgbBrightnessHandler(store, cfg, cfgPath, cfgMu),
	}
}

func persistRGB(cfg *config.Config, cfgPath string, mutate func(*config.RGB)) error {
	updated := *cfg
	mutate(&updated.RGB)
	if err := updated.Save(cfgPath); err != nil {
		return err
	}
	*cfg = updated
	return nil
}

func rgbSetHandler(store *rgb.Store, cfg *config.Config, cfgPath string, opMu *sync.Mutex, enabled bool) ipc.Handler {
	return func(args map[string]any) (any, error) {
		opMu.Lock()
		defer opMu.Unlock()

		if err := persistRGB(cfg, cfgPath, func(rgbCfg *config.RGB) { rgbCfg.Enabled = enabled }); err != nil {
			return nil, fmt.Errorf("persist RGB state: %w", err)
		}

		var (
			state rgb.State
			err   error
		)
		if enabled {
			state, err = store.On()
		} else {
			state, err = store.Off()
		}
		if err != nil {
			return nil, fmt.Errorf("apply RGB state: %w", err)
		}

		return map[string]bool{"enabled": state.Enabled}, nil
	}
}

func rgbColorHandler(store *rgb.Store, cfg *config.Config, cfgPath string, opMu *sync.Mutex) ipc.Handler {
	return func(args map[string]any) (any, error) {
		hex, ok := args["hex"].(string)
		if !ok {
			return nil, fmt.Errorf("rgb.color: missing \"hex\" argument")
		}
		if _, _, _, err := rgb.ParseColor(hex); err != nil {
			return nil, err
		}

		opMu.Lock()
		defer opMu.Unlock()

		if err := persistRGB(cfg, cfgPath, func(rgbCfg *config.RGB) { rgbCfg.Color = hex }); err != nil {
			return nil, fmt.Errorf("persist RGB color: %w", err)
		}

		state, err := store.SetColor(hex)
		if err != nil {
			return nil, fmt.Errorf("apply RGB color: %w", err)
		}

		return map[string]string{"color": state.Color}, nil
	}
}

func rgbBrightnessHandler(store *rgb.Store, cfg *config.Config, cfgPath string, opMu *sync.Mutex) ipc.Handler {
	return func(args map[string]any) (any, error) {
		raw, ok := args["percent"].(float64)
		if !ok || math.IsNaN(raw) || math.IsInf(raw, 0) {
			return nil, fmt.Errorf("rgb.brightness: missing or invalid \"percent\" argument")
		}
		percent := int(raw)
		if err := rgb.ValidateBrightness(percent); err != nil {
			return nil, err
		}

		opMu.Lock()
		defer opMu.Unlock()

		if err := persistRGB(cfg, cfgPath, func(rgbCfg *config.RGB) { rgbCfg.Brightness = percent }); err != nil {
			return nil, fmt.Errorf("persist RGB brightness: %w", err)
		}

		state, err := store.SetBrightness(percent)
		if err != nil {
			return nil, fmt.Errorf("apply RGB brightness: %w", err)
		}

		return map[string]int{"brightness": state.Brightness}, nil
	}
}
