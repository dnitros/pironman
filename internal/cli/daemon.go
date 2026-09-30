package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/clock"
	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/fan"
	"github.com/dnitros/pironman/internal/groupaccess"
	"github.com/dnitros/pironman/internal/handlers"
	"github.com/dnitros/pironman/internal/hardware"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/oled"
	"github.com/dnitros/pironman/internal/powerbutton"
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

const powerButtonTickInterval = 50 * time.Millisecond

func dispatchPowerButtonEvent(event powerbutton.Event, oledMachine *oled.Machine, shutdowner hardware.Shutdowner) error {
	switch event {
	case powerbutton.EventClick:
		return oledMachine.Advance()
	case powerbutton.EventDoubleClick:
		return oledMachine.Previous()
	case powerbutton.EventLongPress:
		return oledMachine.ShowShutdownConfirmation()
	case powerbutton.EventLongPressReleased:
		renderErr := oledMachine.ShowPoweringOff()
		shutdownErr := shutdowner.Shutdown()
		return errors.Join(renderErr, shutdownErr)
	default:
		return nil
	}
}

func runPowerButtonWatcher(watcher hardware.PowerButtonWatcher, classifier *powerbutton.Classifier, oledMachine *oled.Machine, shutdowner hardware.Shutdowner, done chan<- struct{}) {
	defer close(done)

	for {
		ev, err := watcher.Next()
		if err != nil {
			return
		}

		var event powerbutton.Event
		if ev.Pressed {
			event = classifier.PressDown()
		} else {
			event = classifier.PressUp()
		}
		if err := dispatchPowerButtonEvent(event, oledMachine, shutdowner); err != nil {
			log.Printf("daemon: power-button dispatch failed: %v", err)
		}
	}
}

func startPowerButtonWatchLoop(watcher hardware.PowerButtonWatcher, classifier *powerbutton.Classifier, oledMachine *oled.Machine, shutdowner hardware.Shutdowner) (shutdown func() error) {
	done := make(chan struct{})
	go runPowerButtonWatcher(watcher, classifier, oledMachine, shutdowner, done)
	return func() error {
		err := watcher.Close()
		<-done
		return err
	}
}

func runTickLoop(ctx context.Context, interval time.Duration, label string, tick func() error, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := tick(); err != nil {
				log.Printf("daemon: %s tick failed: %v", label, err)
			}
		}
	}
}

func startTickLoop(ctx context.Context, interval time.Duration, label string, tick func() error) (shutdown func() error) {
	tickCtx, cancelTick := context.WithCancel(ctx)
	tickerDone := make(chan struct{})
	go runTickLoop(tickCtx, interval, label, tick, tickerDone)
	return func() error {
		cancelTick()
		<-tickerDone
		return nil
	}
}

func runDaemon(ctx context.Context) error {
	cfgPath := config.Path()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	fmt.Printf("pironman daemon: loaded config from %s\n", cfgPath)

	rgbStore, err := rgb.NewStoreFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("apply initial RGB state: %w", err)
	}

	oledMachine, err := oled.NewMachineFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("apply initial OLED state: %w", err)
	}

	fanMachine, err := fan.NewMachineFromConfig(cfg)
	if err != nil {
		return fmt.Errorf("apply initial fan state: %w", err)
	}

	powerButtonWatcher, err := hardware.NewEvdevPowerButtonWatcher()
	if err != nil {
		return fmt.Errorf("open power-button watcher: %w", err)
	}
	powerButtonClassifier := powerbutton.NewClassifier(clock.RealClock{})
	shutdowner := hardware.SystemShutdowner{}

	path := ipc.SocketPath()

	pwmFanReader := hardware.NewSysPWMFanReader(hardware.PWMFanCoolingStatePath, hardware.PWMFanHwmonFanInputGlob)

	var cfgMu sync.Mutex
	handlerMap := map[string]ipc.Handler{"ping": handlePing, "status": handlers.StatusHandler(rgbStore, oledMachine, fanMachine, pwmFanReader)}
	maps.Copy(handlerMap, handlers.RGBHandlers(rgbStore, &cfg, cfgPath, &cfgMu))
	maps.Copy(handlerMap, handlers.OLEDHandlers(oledMachine, &cfg, cfgPath, &cfgMu))
	maps.Copy(handlerMap, handlers.FanHandlers(fanMachine, &cfg, cfgPath, &cfgMu))

	srv := ipc.NewServer(handlerMap)
	if err := srv.Listen(path); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	stopOLEDTicker := startTickLoop(ctx, time.Second, "oled", oledMachine.Tick)
	defer stopOLEDTicker()

	stopFanTicker := startTickLoop(ctx, time.Second, "fan", fanMachine.Tick)
	defer stopFanTicker()

	stopPowerButtonWatch := startPowerButtonWatchLoop(powerButtonWatcher, powerButtonClassifier, oledMachine, shutdowner)
	defer stopPowerButtonWatch()

	stopPowerButtonTicker := startTickLoop(ctx, powerButtonTickInterval, "power-button", func() error {
		return dispatchPowerButtonEvent(powerButtonClassifier.Tick(), oledMachine, shutdowner)
	})
	defer stopPowerButtonTicker()

	fmt.Printf("pironman daemon listening on %s\n", path)
	return serveDaemon(ctx, srv,
		stopOLEDTicker,
		stopFanTicker,
		stopPowerButtonWatch,
		stopPowerButtonTicker,
		func() error { return oledMachine.Off() },
		func() error { return fanMachine.Off() },
		func() error {
			_, err := rgbStore.Off()
			return err
		},
	)
}

func serveDaemon(ctx context.Context, srv *ipc.Server, shutdownHooks ...func() error) error {
	serveErr := srv.Serve(ctx)

	for _, hook := range shutdownHooks {
		if err := hook(); err != nil {
			log.Printf("daemon: shutdown hook failed: %v", err)
		}
	}

	return serveErr
}

func handlePing(args map[string]any) (any, error) {
	return map[string]string{"message": "pong"}, nil
}
