package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/systemdunit"
)

const spiDevPath = "/dev/spidev0.0"
const i2cDevPath = "/dev/i2c-1"

type DoctorInfo struct {
	PlatformSupported bool
	Installed         bool
	Active            bool
	Reachable         bool
	UnreachableReason string
	SocketPath        string
	SocketPermissions string
	ConfigPath        string
	ConfigStatus      string
	SPIEnabled        bool
	I2CEnabled        bool
	I2CToolsInstalled bool
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Show daemon reachability, install/running state, socket permissions, and config readability",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := gatherDoctor(ipc.SocketPath(), systemdunit.NewManager(), config.Path(), spiDevPath, i2cDevPath)
			if err != nil {
				return err
			}
			printDoctor(info)
			return nil
		},
	}
}

func gatherDoctor(socketPath string, mgr systemdunit.Manager, cfgPath string, spiPath string, i2cPath string) (DoctorInfo, error) {
	supported := mgr.IsSupported()

	var installed, active bool
	if supported {
		var err error
		installed, err = mgr.IsInstalled()
		if err != nil {
			return DoctorInfo{}, fmt.Errorf("check install state: %w", err)
		}

		active, err = mgr.IsActive()
		if err != nil {
			return DoctorInfo{}, fmt.Errorf("check active state: %w", err)
		}
	}

	reachable, unreachableReason := pingDaemon(socketPath)

	return DoctorInfo{
		PlatformSupported: supported,
		Installed:         installed,
		Active:            active,
		Reachable:         reachable,
		UnreachableReason: unreachableReason,
		SocketPath:        socketPath,
		SocketPermissions: describeSocketPermissions(socketPath),
		ConfigPath:        cfgPath,
		ConfigStatus:      describeConfigStatus(cfgPath),
		SPIEnabled:        devicePathExists(spiPath),
		I2CEnabled:        devicePathExists(i2cPath),
		I2CToolsInstalled: commandInstalled("i2cdetect"),
	}, nil
}

func devicePathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// commandInstalled is a convenience check only — pironman talks to the I2C
// bus directly via periph.io, not through i2c-tools — useful for a human
// debugging further.
func commandInstalled(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func pingDaemon(socketPath string) (ok bool, message string) {
	resp, err := ipc.Send(socketPath, "ping", nil)
	switch {
	case err != nil:
		return false, err.Error()
	case !resp.OK:
		return false, resp.Error
	default:
		return true, ""
	}
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

func printDoctor(info DoctorInfo) {
	if !info.PlatformSupported {
		fmt.Println("platform: unsupported (systemctl not found — pironman requires a Linux system with systemd)")
	} else {
		fmt.Println("platform: supported")
		fmt.Printf("installed: %t\n", info.Installed)
		fmt.Printf("active: %t\n", info.Active)
	}
	if info.Reachable {
		fmt.Println("daemon: reachable")
	} else {
		fmt.Printf("daemon: unreachable (%s)\n", info.UnreachableReason)
	}
	fmt.Printf("socket path: %s\n", info.SocketPath)
	fmt.Printf("socket permissions: %s\n", info.SocketPermissions)
	fmt.Printf("config path: %s\n", info.ConfigPath)
	fmt.Printf("config: %s\n", info.ConfigStatus)
	fmt.Printf("SPI enabled: %t\n", info.SPIEnabled)
	fmt.Printf("I2C enabled: %t\n", info.I2CEnabled)
	fmt.Printf("i2c-tools installed: %t\n", info.I2CToolsInstalled)
}
