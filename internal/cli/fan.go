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
	cmd.AddCommand(newFanModeCmd())
	return cmd
}

func newFanModeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mode <name>",
		Short: "Set the case fan's temperature curve (always_on, performance, cool, balanced, quiet)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFanMode(ipc.SocketPath(), args[0])
		},
	}
}

func runFanMode(socketPath, name string) error {
	if _, err := sendCommand(socketPath, "fan.mode", map[string]any{"name": name}, "fan mode "+name); err != nil {
		return err
	}

	fmt.Printf("case fan mode set to %s\n", name)
	return nil
}
