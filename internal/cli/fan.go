package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/ipc"
)

func newFanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fan",
		Short: "Control the case fan",
	}
	cmd.AddCommand(newFanSetCmd("on", "Turn the case fan on"))
	cmd.AddCommand(newFanSetCmd("off", "Turn the case fan off"))
	cmd.AddCommand(newFanSetCmd("auto", "Let the case fan follow CPU temperature automatically"))
	return cmd
}

func newFanSetCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFanSet(ipc.SocketPath(), use)
		},
	}
}

func runFanSet(socketPath, use string) error {
	if _, err := sendCommand(socketPath, "fan."+use, nil, "fan "+use); err != nil {
		return err
	}

	fmt.Printf("case fan set to %s\n", use)
	return nil
}
