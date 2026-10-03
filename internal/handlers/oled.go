package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/imageconv"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/oled"
)

func OLEDHandlers(machine *oled.Machine, cfg *config.Config, cfgPath string, cfgMu *sync.Mutex) map[string]ipc.Handler {
	return map[string]ipc.Handler{
		"oled.on":            oledSetHandler(machine, cfg, cfgPath, cfgMu, true),
		"oled.off":           oledSetHandler(machine, cfg, cfgPath, cfgMu, false),
		"oled.page":          oledPageHandler(machine, cfg, cfgPath, cfgMu),
		"oled.image":         oledImageHandler(machine, cfg, cfgPath, cfgMu),
		"oled.rotation":      oledRotationHandler(machine, cfg, cfgPath, cfgMu),
		"oled.sleep-timeout": oledSleepTimeoutHandler(machine, cfg, cfgPath, cfgMu),
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

func oledImageHandler(machine *oled.Machine, cfg *config.Config, cfgPath string, opMu *sync.Mutex) ipc.Handler {
	return func(args map[string]any) (any, error) {
		paths, err := stringSliceArg(args["paths"])
		if err != nil {
			return nil, fmt.Errorf("oled.image: %w", err)
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("oled.image: at least one path is required")
		}

		opMu.Lock()
		defer opMu.Unlock()

		if !containsPage(cfg.OLED.PageOrder, oled.PageImage) {
			return nil, fmt.Errorf("oled.image: %q is not in oled.page_order; add it to config.yaml and restart the daemon", oled.PageImage)
		}

		interval := cfg.OLED.ImageIntervalSeconds
		if v, ok := args["interval"].(float64); ok && v > 0 {
			interval = int(v)
		}
		invert, _ := args["invert"].(bool)

		imagesDir := filepath.Join(filepath.Dir(cfgPath), "images")
		stored := make([]string, 0, len(paths))
		for i, p := range paths {
			dest, err := imageconv.PersistImage(p, imagesDir, fmt.Sprintf("image-%d", i), invert)
			if err != nil {
				return nil, fmt.Errorf("oled.image: %w", err)
			}
			stored = append(stored, dest)
		}

		previous := cfg.OLED.ImagePaths
		if err := persistOLEDImage(cfg, cfgPath, stored, interval); err != nil {
			return nil, fmt.Errorf("persist OLED images: %w", err)
		}
		removeOrphanedImages(previous, stored)

		if err := machine.SetImages(stored, time.Duration(interval)*time.Second); err != nil {
			return nil, fmt.Errorf("apply OLED images: %w", err)
		}
		if err := machine.SetPage(oled.PageImage); err != nil {
			return nil, fmt.Errorf("apply OLED page: %w", err)
		}

		state := machine.State()
		return map[string]any{"awake": state.Awake, "page": state.Page, "paths": stored}, nil
	}
}

func persistOLEDRotation(cfg *config.Config, cfgPath string, degrees int) error {
	updated := *cfg
	updated.OLED.Rotation = degrees
	if err := updated.Save(cfgPath); err != nil {
		return err
	}
	*cfg = updated
	return nil
}

func oledRotationHandler(machine *oled.Machine, cfg *config.Config, cfgPath string, opMu *sync.Mutex) ipc.Handler {
	return func(args map[string]any) (any, error) {
		v, ok := args["degrees"].(float64)
		if !ok {
			return nil, fmt.Errorf("oled.rotation: missing \"degrees\" argument")
		}
		degrees := int(v)
		if err := hardware.ValidateRotation(degrees); err != nil {
			return nil, err
		}

		opMu.Lock()
		defer opMu.Unlock()

		if err := persistOLEDRotation(cfg, cfgPath, degrees); err != nil {
			return nil, fmt.Errorf("persist OLED rotation: %w", err)
		}
		if err := machine.SetRotation(degrees); err != nil {
			return nil, fmt.Errorf("apply OLED rotation: %w", err)
		}

		return map[string]any{"rotation": degrees}, nil
	}
}

func persistOLEDSleepTimeout(cfg *config.Config, cfgPath string, seconds int) error {
	updated := *cfg
	updated.OLED.SleepTimeoutSeconds = seconds
	if err := updated.Save(cfgPath); err != nil {
		return err
	}
	*cfg = updated
	return nil
}

func oledSleepTimeoutHandler(machine *oled.Machine, cfg *config.Config, cfgPath string, opMu *sync.Mutex) ipc.Handler {
	return func(args map[string]any) (any, error) {
		v, ok := args["seconds"].(float64)
		if !ok {
			return nil, fmt.Errorf("oled.sleep-timeout: missing \"seconds\" argument")
		}
		seconds := int(v)
		if err := oled.ValidateSleepTimeoutSeconds(seconds); err != nil {
			return nil, err
		}

		opMu.Lock()
		defer opMu.Unlock()

		if err := persistOLEDSleepTimeout(cfg, cfgPath, seconds); err != nil {
			return nil, fmt.Errorf("persist OLED sleep-timeout: %w", err)
		}
		if err := machine.SetSleepTimeout(seconds); err != nil {
			return nil, fmt.Errorf("apply OLED sleep-timeout: %w", err)
		}

		return map[string]any{"sleep_timeout_seconds": seconds}, nil
	}
}

func persistOLEDImage(cfg *config.Config, cfgPath string, paths []string, intervalSeconds int) error {
	updated := *cfg
	updated.OLED.Enabled = true
	updated.OLED.ImagePaths = paths
	updated.OLED.ImageIntervalSeconds = intervalSeconds
	if err := updated.Save(cfgPath); err != nil {
		return err
	}
	*cfg = updated
	return nil
}

func removeOrphanedImages(previous, stored []string) {
	for _, p := range previous {
		if !slices.Contains(stored, p) {
			_ = os.Remove(p)
		}
	}
}

func stringSliceArg(v any) ([]string, error) {
	raw, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("missing or invalid \"paths\" argument")
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("\"paths\" must be a list of strings")
		}
		out = append(out, s)
	}
	return out, nil
}

func containsPage(pages []string, name string) bool {
	for _, p := range pages {
		if p == name {
			return true
		}
	}
	return false
}
