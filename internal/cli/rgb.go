package cli

import (
	"fmt"
	"strconv"

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
	cmd.AddCommand(newRGBColorCmd())
	cmd.AddCommand(newRGBBrightnessCmd())
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

func newRGBColorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "color <#hex>",
		Short: "Set the RGB strip to a solid color",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRGBColor(ipc.SocketPath(), args[0])
		},
	}
}

func runRGBColor(socketPath, hex string) error {
	resp, err := ipc.Send(socketPath, "rgb.color", map[string]any{"hex": hex})
	if err != nil {
		return fmt.Errorf("rgb color: daemon unreachable: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("rgb color: %s", resp.Error)
	}

	fmt.Printf("RGB strip color set to %s\n", hex)
	return nil
}

func newRGBBrightnessCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "brightness <0-100>",
		Short:              "Set the RGB strip's brightness",
		Args:               cobra.ExactArgs(1),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "-h" || args[0] == "--help" {
				return cmd.Help()
			}
			percent, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("rgb brightness: %q is not a valid integer", args[0])
			}
			return runRGBBrightness(ipc.SocketPath(), percent)
		},
	}
}

func runRGBBrightness(socketPath string, percent int) error {
	resp, err := ipc.Send(socketPath, "rgb.brightness", map[string]any{"percent": percent})
	if err != nil {
		return fmt.Errorf("rgb brightness: daemon unreachable: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("rgb brightness: %s", resp.Error)
	}

	fmt.Printf("RGB strip brightness set to %d\n", percent)
	return nil
}
