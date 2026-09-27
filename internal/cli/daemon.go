package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/groupaccess"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/systemdunit"
)

var geteuid = os.Geteuid

// daemonCommand is a Command-pattern entry (receiver: systemdunit.Manager):
// it binds a call against the receiver to the guard requirements and
// success reporting runDaemonCommand applies without knowing which command
// it's running.
type daemonCommand struct {
	use              string
	short            string
	name             string // used in guard error messages, e.g. "daemon start"
	requireInstalled bool
	execute          func(mgr systemdunit.Manager) error
	message          string
	after            func() // optional; runs after message is printed
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

// runDaemonCommand is the Command-pattern invoker: it applies the guard
// sequence every daemon lifecycle command needs, then runs c without
// knowing which one it is.
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
	if _, err := config.Load(cfgPath); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	fmt.Printf("pironman daemon: loaded config from %s\n", cfgPath)

	path := ipc.SocketPath()

	srv := ipc.NewServer(map[string]ipc.Handler{
		"ping": handlePing,
	})
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
