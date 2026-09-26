package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/systemdunit"
)

var geteuid = os.Geteuid

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the pironman daemon",
	}
	cmd.AddCommand(newDaemonRunCmd())
	cmd.AddCommand(newDaemonInstallCmd())
	cmd.AddCommand(newDaemonUninstallCmd())
	cmd.AddCommand(newDaemonStartCmd())
	cmd.AddCommand(newDaemonStopCmd())
	return cmd
}

func newDaemonInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the pironman systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemonInstall(systemdunit.NewManager())
		},
	}
}

func newDaemonUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall the pironman systemd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemonUninstall(systemdunit.NewManager())
		},
	}
}

func newDaemonStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the installed pironman service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemonStart(systemdunit.NewManager())
		},
	}
}

func newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the installed pironman service",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemonStop(systemdunit.NewManager())
		},
	}
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

func runDaemonInstall(mgr systemdunit.Manager) error {
	if err := requireRoot("daemon install"); err != nil {
		return err
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve pironman binary path: %w", err)
	}

	if err := mgr.Install(systemdunit.UnitContent(execPath)); err != nil {
		return fmt.Errorf("install service: %w", err)
	}

	fmt.Println("pironman service installed")
	return nil
}

func runDaemonUninstall(mgr systemdunit.Manager) error {
	if err := requireRoot("daemon uninstall"); err != nil {
		return err
	}

	if err := mgr.Uninstall(); err != nil {
		return fmt.Errorf("uninstall service: %w", err)
	}

	fmt.Println("pironman service uninstalled")
	return nil
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

func runDaemonStart(mgr systemdunit.Manager) error {
	if err := requireInstalled(mgr, "daemon start"); err != nil {
		return err
	}

	if err := mgr.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}

	fmt.Println("pironman service started")
	return nil
}

func runDaemonStop(mgr systemdunit.Manager) error {
	if err := requireInstalled(mgr, "daemon stop"); err != nil {
		return err
	}

	if err := mgr.Stop(); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}

	fmt.Println("pironman service stopped")
	return nil
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
