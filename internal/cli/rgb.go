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
	cmd.AddCommand(newRGBStyleCmd())
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
	if _, err := sendCommand(socketPath, "rgb."+use, nil, "rgb "+use); err != nil {
		return err
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
	if _, err := sendCommand(socketPath, "rgb.color", map[string]any{"hex": hex}, "rgb color"); err != nil {
		return err
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
	if _, err := sendCommand(socketPath, "rgb.brightness", map[string]any{"percent": percent}, "rgb brightness"); err != nil {
		return err
	}

	fmt.Printf("RGB strip brightness set to %d\n", percent)
	return nil
}

func newRGBStyleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "style <name>",
		Short: "Set the RGB strip's lighting style",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var speed *int
			if cmd.Flags().Changed("speed") {
				v, err := cmd.Flags().GetInt("speed")
				if err != nil {
					return err
				}
				speed = &v
			}
			return runRGBStyle(ipc.SocketPath(), args[0], speed)
		},
	}
	cmd.Flags().Int("speed", 0, "animation speed (0-100); leaves the configured speed unchanged if omitted")
	return cmd
}

func runRGBStyle(socketPath, name string, speed *int) error {
	reqArgs := map[string]any{"name": name}
	if speed != nil {
		reqArgs["speed"] = *speed
	}
	if _, err := sendCommand(socketPath, "rgb.style", reqArgs, "rgb style"); err != nil {
		return err
	}

	fmt.Printf("RGB strip style set to %s\n", name)
	return nil
}
