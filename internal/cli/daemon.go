package cli

import (
	"context"
	"fmt"
	"maps"
	"math"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/groupaccess"
	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/rgb"
	"github.com/dnitros/pironman/internal/systemdunit"
)

var geteuid = os.Geteuid

type daemonCommand struct {
	use              string
	short            string
	name             string
	requireInstalled bool
	execute          func(mgr systemdunit.Manager) error
	message          string
	after            func()
}

var daemonCommands = []daemonCommand{
	{
		use:   "install",
		short: "Install the pironman systemd service",
		name:  "daemon install",
		execute: func(mgr systemdunit.Manager) error {
			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve pironman binary path: %w", err)
			}
			if err := mgr.Install(systemdunit.UnitContent(execPath)); err != nil {
				return fmt.Errorf("install service: %w", err)
			}
			return nil
		},
		message: "pironman service installed",
		after:   installGroupJoinHook,
	},
	{
		use:   "uninstall",
		short: "Uninstall the pironman systemd service",
		name:  "daemon uninstall",
		execute: func(mgr systemdunit.Manager) error {
			if err := mgr.Uninstall(); err != nil {
				return fmt.Errorf("uninstall service: %w", err)
			}
			return nil
		},
		message: "pironman service uninstalled",
	},
	{
		use:              "start",
		short:            "Start the installed pironman service",
		name:             "daemon start",
		requireInstalled: true,
		execute: func(mgr systemdunit.Manager) error {
			if err := mgr.Start(); err != nil {
				return fmt.Errorf("start service: %w", err)
			}
			return nil
		},
		message: "pironman service started",
	},
	{
		use:              "stop",
		short:            "Stop the installed pironman service",
		name:             "daemon stop",
		requireInstalled: true,
		execute: func(mgr systemdunit.Manager) error {
			if err := mgr.Stop(); err != nil {
				return fmt.Errorf("stop service: %w", err)
			}
			return nil
		},
		message: "pironman service stopped",
	},
	{
		use:              "enable",
		short:            "Enable the pironman service to start automatically on boot",
		name:             "daemon enable",
		requireInstalled: true,
		execute: func(mgr systemdunit.Manager) error {
			if err := mgr.Enable(); err != nil {
				return fmt.Errorf("enable service: %w", err)
			}
			return nil
		},
		message: "pironman service enabled — it will start automatically on boot",
	},
	{
		use:              "disable",
		short:            "Disable automatic startup of the pironman service on boot",
		name:             "daemon disable",
		requireInstalled: true,
		execute: func(mgr systemdunit.Manager) error {
			if err := mgr.Disable(); err != nil {
				return fmt.Errorf("disable service: %w", err)
			}
			return nil
		},
		message: "pironman service disabled — it will not start automatically on boot",
	},
}

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the pironman daemon",
	}
	cmd.AddCommand(newDaemonRunCmd())
	for _, c := range daemonCommands {
		cmd.AddCommand(&cobra.Command{
			Use:   c.use,
			Short: c.short,
			RunE: func(cmd *cobra.Command, args []string) error {
				return runDaemonCommand(systemdunit.NewManager(), c)
			},
		})
	}
	return cmd
}

func requireRoot(action string) error {
	if geteuid() != 0 {
		return fmt.Errorf("%s requires root — rerun with sudo", action)
	}
	return nil
}

func errNotInstalled(action string) error {
	return fmt.Errorf("%s: pironman service is not installed — run `pironman daemon install` first", action)
}

func requireSupported(mgr systemdunit.Manager, action string) error {
	if mgr.IsSupported() {
		return nil
	}
	return fmt.Errorf("%s: pironman requires a Linux system with systemd (systemctl not found) — unsupported on this machine", action)
}

func requireInstalled(mgr systemdunit.Manager, action string) error {
	installed, err := mgr.IsInstalled()
	if err != nil {
		return fmt.Errorf("check install state: %w", err)
	}
	if !installed {
		return errNotInstalled(action)
	}
	return nil
}

func runDaemonCommand(mgr systemdunit.Manager, c daemonCommand) error {
	if err := requireSupported(mgr, c.name); err != nil {
		return err
	}
	if err := requireRoot(c.name); err != nil {
		return err
	}
	if c.requireInstalled {
		if err := requireInstalled(mgr, c.name); err != nil {
			return err
		}
	}

	if err := c.execute(mgr); err != nil {
		return err
	}

	fmt.Println(c.message)

	if c.after != nil {
		c.after()
	}
	return nil
}

func installGroupJoinHook() {
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		fmt.Println(groupJoinHint())
		return
	}

	if alreadyMember, _ := groupaccess.IsMember(sudoUser); alreadyMember {
		return
	}

	if err := groupaccess.AddMember(sudoUser); err != nil {
		fmt.Printf("could not add %s to the %s group automatically: %v\n", sudoUser, ipc.GroupName, err)
		fmt.Println(groupJoinHint())
		return
	}

	fmt.Printf("added %s to the %s group — log out and back in for it to take effect\n", sudoUser, ipc.GroupName)
}

func groupJoinHint() string {
	user := os.Getenv("SUDO_USER")
	if user == "" {
		user = "<your-username>"
	}
	return fmt.Sprintf("to use the CLI without sudo, run: sudo usermod -aG %s %s (then log out and back in for it to take effect)", ipc.GroupName, user)
}

func newDaemonRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "run",
		Short:  "Run the pironman daemon in the foreground",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon(cmd.Context())
		},
	}
}

func runDaemon(ctx context.Context) error {
	cfgPath := config.Path()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	fmt.Printf("pironman daemon: loaded config from %s\n", cfgPath)

	r, g, b, err := rgb.ScaledColor(cfg.RGB.Color, cfg.RGB.Brightness)
	if err != nil {
		return fmt.Errorf("parse configured RGB color/brightness: %w", err)
	}
	strip, err := hardware.NewSPIWS2812(hardware.SPIPort, hardware.NumLEDs, r, g, b)
	if err != nil {
		return fmt.Errorf("open WS2812 strip: %w", err)
	}
	rgbStore, err := rgb.NewStore(strip, rgb.State{Enabled: cfg.RGB.Enabled, Color: cfg.RGB.Color, Brightness: cfg.RGB.Brightness})
	if err != nil {
		return fmt.Errorf("apply initial RGB state: %w", err)
	}

	path := ipc.SocketPath()

	handlers := map[string]ipc.Handler{"ping": handlePing, "status": statusHandler(rgbStore)}
	maps.Copy(handlers, rgbHandlers(rgbStore, &cfg, cfgPath))

	srv := ipc.NewServer(handlers)
	if err := srv.Listen(path); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("pironman daemon listening on %s\n", path)
	return srv.Serve(ctx)
}

func handlePing(args map[string]any) (any, error) {
	return map[string]string{"message": "pong"}, nil
}

func statusHandler(store *rgb.Store) ipc.Handler {
	return func(args map[string]any) (any, error) {
		state := store.State()
		return map[string]any{
			"enabled":    state.Enabled,
			"color":      state.Color,
			"brightness": state.Brightness,
		}, nil
	}
}

// rgbHandlers registers the rgb.on/rgb.off/rgb.color/rgb.brightness IPC
// commands against store, persisting the resulting state to cfg/cfgPath on
// every call. cfg and
// cfgPath are shared across concurrent IPC connections; opMu serializes each
// call's persist together with its store mutation so the two can't reorder
// relative to a competing call. Persist always happens before the strip is
// touched, so cfg/disk (and a subsequent restart's rgb.NewStore) reflect the
// intended state regardless of whether the strip write itself succeeds.
func rgbHandlers(store *rgb.Store, cfg *config.Config, cfgPath string) map[string]ipc.Handler {
	var opMu sync.Mutex
	return map[string]ipc.Handler{
		"rgb.on":         rgbSetHandler(store, cfg, cfgPath, &opMu, true),
		"rgb.off":        rgbSetHandler(store, cfg, cfgPath, &opMu, false),
		"rgb.color":      rgbColorHandler(store, cfg, cfgPath, &opMu),
		"rgb.brightness": rgbBrightnessHandler(store, cfg, cfgPath, &opMu),
	}
}

// persistRGB copies cfg, applies mutate to the copy's RGB section, saves the
// copy, and swaps it into cfg only on success — the persist-before-hardware-
// write step shared by every rgb.* handler below.
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

// rgbColorHandler validates the "hex" arg before persisting it.
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

// rgbBrightnessHandler mirrors rgbColorHandler, validating the "percent" arg
// before persisting it. JSON numbers decode to float64, so percent arrives
// as a float64 even though the CLI sends an int.
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
