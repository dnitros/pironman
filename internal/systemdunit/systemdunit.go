package systemdunit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/dnitros/pironman/internal/groupaccess"
)

const (
	UnitPath    = "/etc/systemd/system/pironman.service"
	ServiceName = "pironman"
)

const unitTemplate = `[Unit]
Description=Pironman 5 case daemon
After=network.target

[Service]
ExecStart=%s daemon run
Restart=on-failure

[Install]
WantedBy=multi-user.target
`

func UnitContent(execPath string) string {
	return fmt.Sprintf(unitTemplate, execPath)
}

type Manager interface {
	Install(unitContent string) error
	Uninstall() error
	Start() error
	Stop() error
	Restart() error
	Enable() error
	Disable() error
	IsInstalled() (bool, error)
	IsActive() (bool, error)
	IsSupported() bool
}

type SystemdManager struct{}

func NewManager() Manager {
	return SystemdManager{}
}

func (SystemdManager) IsSupported() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func (SystemdManager) IsInstalled() (bool, error) {
	_, err := os.Stat(UnitPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", UnitPath, err)
	}
	return true, nil
}

func (SystemdManager) Install(unitContent string) error {
	if err := groupaccess.EnsureGroup(); err != nil {
		return err
	}

	if err := os.WriteFile(UnitPath, []byte(unitContent), 0o644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}

	return runSystemctl("daemon-reload")
}

func (SystemdManager) Uninstall() error {
	_ = runSystemctl("stop", ServiceName)
	_ = runSystemctl("disable", ServiceName)

	if err := os.Remove(UnitPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove unit file: %w", err)
	}
	return runSystemctl("daemon-reload")
}

func (SystemdManager) IsActive() (bool, error) {
	err := exec.Command("systemctl", "is-active", "--quiet", ServiceName).Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// ponytail: any non-zero exit (including a dbus/policy failure, not just a
		// genuinely stopped unit) is reported as inactive; upgrade to inspecting
		// systemctl's printed state word if that distinction starts to matter.
		return false, nil
	}
	return false, fmt.Errorf("run systemctl is-active: %w", err)
}

func (SystemdManager) Start() error {
	return runSystemctl("start", ServiceName)
}

func (SystemdManager) Stop() error {
	return runSystemctl("stop", ServiceName)
}

func (SystemdManager) Restart() error {
	return runSystemctl("restart", ServiceName)
}

func (SystemdManager) Enable() error {
	return runSystemctl("enable", ServiceName)
}

func (SystemdManager) Disable() error {
	return runSystemctl("disable", ServiceName)
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
