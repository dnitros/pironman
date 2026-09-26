package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
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
	cmd.AddCommand(newDaemonEnableCmd())
	cmd.AddCommand(newDaemonDisableCmd())
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

func newDaemonEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable the pironman service to start automatically on boot",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemonEnable(systemdunit.NewManager())
		},
	}
}

func newDaemonDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable automatic startup of the pironman service on boot",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemonDisable(systemdunit.NewManager())
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

func requireSupported(mgr systemdunit.Manager, action string) error {
	if mgr.IsSupported() {
		return nil
	}
	return fmt.Errorf("%s: pironman requires a Linux system with systemd (systemctl not found) — unsupported on this machine", action)
}

func runDaemonInstall(mgr systemdunit.Manager) error {
	if err := requireSupported(mgr, "daemon install"); err != nil {
		return err
	}
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

	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		fmt.Println(groupJoinHint())
		return nil
	}

	if alreadyMember, _ := isUserInGroup(sudoUser, ipc.GroupName); alreadyMember {
		return nil
	}

	if err := addUserToGroup(sudoUser); err != nil {
		fmt.Printf("could not add %s to the %s group automatically: %v\n", sudoUser, ipc.GroupName, err)
		fmt.Println(groupJoinHint())
		return nil
	}

	fmt.Printf("added %s to the %s group — log out and back in for it to take effect\n", sudoUser, ipc.GroupName)
	return nil
}

var addUserToGroup = func(username string) error {
	if err := exec.Command("usermod", "-aG", ipc.GroupName, username).Run(); err != nil {
		return fmt.Errorf("usermod -aG %s %s: %w", ipc.GroupName, username, err)
	}
	return nil
}

var isUserInGroup = func(username, groupName string) (bool, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return false, fmt.Errorf("look up user %s: %w", username, err)
	}
	g, err := user.LookupGroup(groupName)
	if err != nil {
		return false, fmt.Errorf("look up group %s: %w", groupName, err)
	}
	gids, err := u.GroupIds()
	if err != nil {
		return false, fmt.Errorf("look up groups for %s: %w", username, err)
	}
	for _, gid := range gids {
		if gid == g.Gid {
			return true, nil
		}
	}
	return false, nil
}

func groupJoinHint() string {
	user := os.Getenv("SUDO_USER")
	if user == "" {
		user = "<your-username>"
	}
	return fmt.Sprintf("to use the CLI without sudo, run: sudo usermod -aG %s %s (then log out and back in for it to take effect)", ipc.GroupName, user)
}

func runDaemonUninstall(mgr systemdunit.Manager) error {
	if err := requireSupported(mgr, "daemon uninstall"); err != nil {
		return err
	}
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
	if err := requireSupported(mgr, "daemon start"); err != nil {
		return err
	}
	if err := requireRoot("daemon start"); err != nil {
		return err
	}
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
	if err := requireSupported(mgr, "daemon stop"); err != nil {
		return err
	}
	if err := requireRoot("daemon stop"); err != nil {
		return err
	}
	if err := requireInstalled(mgr, "daemon stop"); err != nil {
		return err
	}

	if err := mgr.Stop(); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}

	fmt.Println("pironman service stopped")
	return nil
}

func runDaemonEnable(mgr systemdunit.Manager) error {
	if err := requireSupported(mgr, "daemon enable"); err != nil {
		return err
	}
	if err := requireRoot("daemon enable"); err != nil {
		return err
	}
	if err := requireInstalled(mgr, "daemon enable"); err != nil {
		return err
	}

	if err := mgr.Enable(); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}

	fmt.Println("pironman service enabled — it will start automatically on boot")
	return nil
}

func runDaemonDisable(mgr systemdunit.Manager) error {
	if err := requireSupported(mgr, "daemon disable"); err != nil {
		return err
	}
	if err := requireRoot("daemon disable"); err != nil {
		return err
	}
	if err := requireInstalled(mgr, "daemon disable"); err != nil {
		return err
	}

	if err := mgr.Disable(); err != nil {
		return fmt.Errorf("disable service: %w", err)
	}

	fmt.Println("pironman service disabled — it will not start automatically on boot")
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
