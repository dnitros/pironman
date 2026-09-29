package handlers

import (
	"fmt"
	"sync"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/oled"
)

func OLEDHandlers(machine *oled.Machine, cfg *config.Config, cfgPath string, cfgMu *sync.Mutex) map[string]ipc.Handler {
	return map[string]ipc.Handler{
		"oled.on":   oledSetHandler(machine, cfg, cfgPath, cfgMu, true),
		"oled.off":  oledSetHandler(machine, cfg, cfgPath, cfgMu, false),
		"oled.page": oledPageHandler(machine, cfg, cfgPath, cfgMu),
	}
}

func persistOLEDEnabled(cfg *config.Config, cfgPath string, enabled bool) error {
	updated := *cfg
	updated.OLED.Enabled = enabled
	if err := updated.Save(cfgPath); err != nil {
		return err
	}
	*cfg = updated
	return nil
}

func oledSetHandler(machine *oled.Machine, cfg *config.Config, cfgPath string, opMu *sync.Mutex, enabled bool) ipc.Handler {
	return func(args map[string]any) (any, error) {
		opMu.Lock()
		defer opMu.Unlock()

		if err := persistOLEDEnabled(cfg, cfgPath, enabled); err != nil {
			return nil, fmt.Errorf("persist OLED state: %w", err)
		}

		var err error
		if enabled {
			err = machine.On()
		} else {
			err = machine.Off()
		}
		if err != nil {
			return nil, fmt.Errorf("apply OLED state: %w", err)
		}

		return map[string]bool{"awake": machine.State().Awake}, nil
	}
}

func oledPageHandler(machine *oled.Machine, cfg *config.Config, cfgPath string, opMu *sync.Mutex) ipc.Handler {
	return func(args map[string]any) (any, error) {
		page, ok := args["page"].(string)
		if !ok || page == "" {
			return nil, fmt.Errorf("oled.page: missing \"page\" argument")
		}

		opMu.Lock()
		defer opMu.Unlock()

		switch page {
		case "prev":
			if err := machine.Previous(); err != nil {
				return nil, fmt.Errorf("apply OLED page: %w", err)
			}
		case "next":
			if err := persistOLEDEnabled(cfg, cfgPath, true); err != nil {
				return nil, fmt.Errorf("persist OLED state: %w", err)
			}
			if err := machine.Advance(); err != nil {
				return nil, fmt.Errorf("apply OLED page: %w", err)
			}
		default:
			if !containsPage(cfg.OLED.PageOrder, page) {
				return nil, fmt.Errorf("oled.page: unknown page %q", page)
			}
			if err := persistOLEDEnabled(cfg, cfgPath, true); err != nil {
				return nil, fmt.Errorf("persist OLED state: %w", err)
			}
			if err := machine.SetPage(page); err != nil {
				return nil, fmt.Errorf("apply OLED page: %w", err)
			}
		}

		state := machine.State()
		return map[string]any{"awake": state.Awake, "page": state.Page}, nil
	}
}

func containsPage(pages []string, name string) bool {
	for _, p := range pages {
		if p == name {
			return true
		}
	}
	return false
}
