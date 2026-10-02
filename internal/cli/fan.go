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
