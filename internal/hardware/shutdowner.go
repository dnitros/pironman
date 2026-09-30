package hardware

import (
	"fmt"
	"os/exec"
)

type Shutdowner interface {
	Shutdown() error
}

type SystemShutdowner struct{}

func (SystemShutdowner) Shutdown() error {
	if err := exec.Command("shutdown", "-h", "now").Run(); err != nil {
		return fmt.Errorf("shutdown -h now: %w", err)
	}
	return nil
}
