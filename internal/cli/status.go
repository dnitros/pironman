package cli

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/buildinfo"
	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/systemdunit"
)

type StatusInfo struct {
	Version           string
	Installed         bool
	Active            bool
	Reachable         bool
	UnreachableReason string
}

type DoctorInfo struct {
	StatusInfo
	SocketPath        string
	SocketPermissions string
	ConfigPath        string
	ConfigStatus      string
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon reachability, version, and install/running state",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := gatherStatus(ipc.SocketPath(), systemdunit.NewManager())
			if err != nil {
				return err
			}
			printStatus(info)
			return nil
		},
	}
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Show status plus socket permissions and config readability",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := gatherDoctor(ipc.SocketPath(), systemdunit.NewManager(), config.Path())
			if err != nil {
				return err
			}
			printDoctor(info)
			return nil
		},
	}
}

func gatherStatus(socketPath string, mgr systemdunit.Manager) (StatusInfo, error) {
	installed, err := mgr.IsInstalled()
	if err != nil {
		return StatusInfo{}, fmt.Errorf("check install state: %w", err)
	}

	active, err := mgr.IsActive()
	if err != nil {
		return StatusInfo{}, fmt.Errorf("check active state: %w", err)
	}

	info := StatusInfo{
		Version:   buildinfo.Version,
		Installed: installed,
		Active:    active,
	}

	ok, msg := pingDaemon(socketPath)
	info.Reachable = ok
	info.UnreachableReason = msg

	return info, nil
}

func gatherDoctor(socketPath string, mgr systemdunit.Manager, cfgPath string) (DoctorInfo, error) {
	status, err := gatherStatus(socketPath, mgr)
	if err != nil {
		return DoctorInfo{}, err
	}

	return DoctorInfo{
		StatusInfo:        status,
		SocketPath:        socketPath,
		SocketPermissions: describeSocketPermissions(socketPath),
		ConfigPath:        cfgPath,
		ConfigStatus:      describeConfigStatus(cfgPath),
	}, nil
}

func describeSocketPermissions(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return err.Error()
	}

	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fi.Mode().Perm().String()
	}

	owner := strconv.Itoa(int(stat.Uid))
	if u, err := user.LookupId(owner); err == nil {
		owner = u.Username
	}
	group := strconv.Itoa(int(stat.Gid))
	if g, err := user.LookupGroupId(group); err == nil {
		group = g.Name
	}

	return fmt.Sprintf("%s:%s %04o", owner, group, fi.Mode().Perm())
}

func describeConfigStatus(cfgPath string) string {
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		return "missing (daemon uses defaults)"
	}
	if _, err := config.Load(cfgPath); err != nil {
		return fmt.Sprintf("not readable: %v", err)
	}
	return "readable"
}

func printStatus(info StatusInfo) {
	fmt.Printf("version: %s\n", info.Version)
	fmt.Printf("installed: %t\n", info.Installed)
	fmt.Printf("active: %t\n", info.Active)
	if info.Reachable {
		fmt.Println("daemon: reachable")
	} else {
		fmt.Printf("daemon: unreachable (%s)\n", info.UnreachableReason)
	}
}

func printDoctor(info DoctorInfo) {
	printStatus(info.StatusInfo)
	fmt.Printf("socket path: %s\n", info.SocketPath)
	fmt.Printf("socket permissions: %s\n", info.SocketPermissions)
	fmt.Printf("config path: %s\n", info.ConfigPath)
	fmt.Printf("config: %s\n", info.ConfigStatus)
}
