package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/ipc"
)

func newRGBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rgb",
		Short: "Control the RGB LED strip",
	}
	cmd.AddCommand(newRGBSetCmd("on", "Turn the RGB strip on"))
	cmd.AddCommand(newRGBSetCmd("off", "Turn the RGB strip off"))
	return cmd
}

func newRGBSetCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRGBSet(ipc.SocketPath(), use)
		},
	}
}

func runRGBSet(socketPath, use string) error {
	resp, err := ipc.Send(socketPath, "rgb."+use, nil)
	if err != nil {
		return fmt.Errorf("rgb %s: daemon unreachable: %w", use, err)
	}
	if !resp.OK {
		return fmt.Errorf("rgb %s: %s", use, resp.Error)
	}

	fmt.Printf("RGB strip turned %s\n", use)
	return nil
}
