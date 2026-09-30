package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/config"
	"github.com/dnitros/pironman/internal/ipc"
	"github.com/dnitros/pironman/internal/systemdunit"
)

const spiDevPath = "/dev/spidev0.0"
const i2cDevPath = "/dev/i2c-1"
const inputDevicesPath = "/proc/bus/input/devices"

const (
	keyLinePrefix = "B: KEY="
	keyPowerCode  = 116
	bitsPerWord   = 64
)

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
	PowerButtonFound  bool
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Show daemon reachability, install/running state, socket permissions, and config readability",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := gatherDoctor(ipc.SocketPath(), systemdunit.NewManager(), config.Path(), spiDevPath, i2cDevPath, inputDevicesPath)
			if err != nil {
				return err
			}
			printDoctor(info)
			return nil
		},
	}
}

func gatherDoctor(socketPath string, mgr systemdunit.Manager, cfgPath string, spiPath string, i2cPath string, inputDevicesPath string) (DoctorInfo, error) {
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
		SPIEnabled:        devPathExists(spiPath),
		I2CEnabled:        devPathExists(i2cPath),
		I2CToolsInstalled: i2cToolsInstalled(),
		PowerButtonFound:  powerButtonDeviceFound(inputDevicesPath),
	}, nil
}

func devPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func powerButtonDeviceFound(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), keyLinePrefix)
		if ok && hasKeyPowerCapability(value) {
			return true
		}
	}
	return false
}

func hasKeyPowerCapability(value string) bool {
	words := strings.Fields(value)
	idx := len(words) - 1 - keyPowerCode/bitsPerWord
	if idx < 0 {
		return false
	}
	bits, err := strconv.ParseUint(words[idx], 16, 64)
	if err != nil {
		return false
	}
	return bits&(1<<uint(keyPowerCode%bitsPerWord)) != 0
}

func i2cToolsInstalled() bool {
	_, err := exec.LookPath("i2cdetect")
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
	fmt.Println("Daemon")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	if !info.PlatformSupported {
		fmt.Fprintln(w, "  platform:\tunsupported (systemctl not found — pironman requires a Linux system with systemd)")
	} else {
		fmt.Fprintln(w, "  platform:\tsupported")
		fmt.Fprintf(w, "  installed:\t%t\n", info.Installed)
		fmt.Fprintf(w, "  active:\t%t\n", info.Active)
	}
	if info.Reachable {
		fmt.Fprintln(w, "  daemon:\treachable")
	} else {
		fmt.Fprintf(w, "  daemon:\tunreachable (%s)\n", info.UnreachableReason)
	}
	fmt.Fprintf(w, "  socket path:\t%s\n", info.SocketPath)
	fmt.Fprintf(w, "  socket permissions:\t%s\n", info.SocketPermissions)
	w.Flush()

	fmt.Println("Config")
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	fmt.Fprintf(w, "  config path:\t%s\n", info.ConfigPath)
	fmt.Fprintf(w, "  config:\t%s\n", info.ConfigStatus)
	w.Flush()

	fmt.Println("Hardware")
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	fmt.Fprintf(w, "  SPI enabled:\t%t\n", info.SPIEnabled)
	fmt.Fprintf(w, "  I2C enabled:\t%t\n", info.I2CEnabled)
	fmt.Fprintf(w, "  i2c-tools installed:\t%t\n", info.I2CToolsInstalled)
	fmt.Fprintf(w, "  power button device found:\t%t\n", info.PowerButtonFound)
	w.Flush()
}
