package handlers

import (
	"fmt"
	"sync"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/fan"
	"github.com/dnitros/pironman/internal/ipc"
)

func FanHandlers(machine *fan.Machine, cfg *config.Config, cfgPath string, cfgMu *sync.Mutex) map[string]ipc.Handler {
	return map[string]ipc.Handler{
		"fan.on":   fanSetHandler(machine, cfg, cfgPath, cfgMu, fan.ModeOn),
		"fan.off":  fanSetHandler(machine, cfg, cfgPath, cfgMu, fan.ModeOff),
		"fan.mode": fanModeHandler(machine, cfg, cfgPath, cfgMu),
	}
}

func persistFan(cfg *config.Config, cfgPath string, mode string) error {
	updated := *cfg
	updated.Fan.CaseFanState = mode
	if err := updated.Save(cfgPath); err != nil {
		return err
	}
	*cfg = updated
	return nil
}

func fanSetHandler(machine *fan.Machine, cfg *config.Config, cfgPath string, opMu *sync.Mutex, mode string) ipc.Handler {
	return func(args map[string]any) (any, error) {
		opMu.Lock()
		defer opMu.Unlock()

		if err := persistFan(cfg, cfgPath, mode); err != nil {
			return nil, fmt.Errorf("persist fan state: %w", err)
		}

		var err error
		switch mode {
		case fan.ModeOn:
			err = machine.On()
		case fan.ModeOff:
			err = machine.Off()
		}
		if err != nil {
			return nil, fmt.Errorf("apply fan state: %w", err)
		}

		state := machine.State()
		return map[string]any{"mode": state.Mode, "relay_on": state.RelayOn}, nil
	}
}

func fanModeHandler(machine *fan.Machine, cfg *config.Config, cfgPath string, opMu *sync.Mutex) ipc.Handler {
	return func(args map[string]any) (any, error) {
		name, ok := args["name"].(string)
		if !ok {
			return nil, fmt.Errorf("fan.mode: missing \"name\" argument")
		}
		if err := fan.ValidateMode(name); err != nil {
			return nil, err
		}

		opMu.Lock()
		defer opMu.Unlock()

		if err := persistFan(cfg, cfgPath, name); err != nil {
			return nil, fmt.Errorf("persist fan state: %w", err)
		}

		if err := machine.Mode(name); err != nil {
			return nil, fmt.Errorf("apply fan state: %w", err)
		}

		state := machine.State()
		return map[string]any{"mode": state.Mode, "relay_on": state.RelayOn}, nil
	}
}
