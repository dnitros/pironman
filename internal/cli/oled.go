package cli

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/ipc"
)

func newOLEDCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "oled",
		Short: "Control the OLED display",
	}
	cmd.AddCommand(newOLEDSetCmd("on", "Wake the display and show the mix page"))
	cmd.AddCommand(newOLEDSetCmd("off", "Blank the display"))
	cmd.AddCommand(newOLEDPageCmd())
	cmd.AddCommand(newOLEDImageCmd())
	cmd.AddCommand(newOLEDRotationCmd())
	cmd.AddCommand(newOLEDSleepTimeoutCmd())
	return cmd
}

func newOLEDSetCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOLEDSet(ipc.SocketPath(), use)
		},
	}
}

func runOLEDSet(socketPath, use string) error {
	if _, err := sendCommand(socketPath, "oled."+use, nil, "oled "+use); err != nil {
		return err
	}

	fmt.Printf("OLED display turned %s\n", use)
	return nil
}

func newOLEDPageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "page <mix|performance|ips|disk|next|prev>",
		Short: "Switch the OLED page",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOLEDPage(ipc.SocketPath(), args[0])
		},
	}
}

func runOLEDPage(socketPath, page string) error {
	if _, err := sendCommand(socketPath, "oled.page", map[string]any{"page": page}, "oled page "+page); err != nil {
		return err
	}

	fmt.Printf("OLED page set to %s\n", page)
	return nil
}

func newOLEDImageCmd() *cobra.Command {
	var interval int
	var invert bool
	cmd := &cobra.Command{
		Use:   "image [--interval seconds] [--invert] <path>...",
		Short: "Convert, persist, and show one or more images on the OLED image page",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOLEDImage(ipc.SocketPath(), args, interval, invert)
		},
	}
	cmd.Flags().IntVar(&interval, "interval", 0, "seconds between images when rotating multiple paths (default: 5s)")
	cmd.Flags().BoolVar(&invert, "invert", false, "invert which pixels are lit")
	return cmd
}

func runOLEDImage(socketPath string, paths []string, interval int, invert bool) error {
	anyPaths := make([]any, len(paths))
	for i, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("oled image: resolve %q: %w", p, err)
		}
		anyPaths[i] = abs
	}

	args := map[string]any{"paths": anyPaths, "invert": invert}
	if interval > 0 {
		args["interval"] = interval
	}

	if _, err := sendCommand(socketPath, "oled.image", args, "oled image"); err != nil {
		return err
	}

	fmt.Printf("OLED image page set (%d path(s))\n", len(paths))
	return nil
}

func newOLEDRotationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rotation <0|180>",
		Short: "Rotate the OLED display 0 or 180 degrees",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			degrees, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("oled rotation: %q is not a valid integer", args[0])
			}
			return runOLEDRotation(ipc.SocketPath(), degrees)
		},
	}
}

func runOLEDRotation(socketPath string, degrees int) error {
	if _, err := sendCommand(socketPath, "oled.rotation", map[string]any{"degrees": degrees}, "oled rotation"); err != nil {
		return err
	}

	fmt.Printf("OLED rotation set to %d\n", degrees)
	return nil
}

func newOLEDSleepTimeoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sleep-timeout <seconds>",
		Short: "Set how long the display stays awake with no activity (0 disables)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			seconds, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("oled sleep-timeout: %q is not a valid integer", args[0])
			}
			return runOLEDSleepTimeout(ipc.SocketPath(), seconds)
		},
	}
}

func runOLEDSleepTimeout(socketPath string, seconds int) error {
	if _, err := sendCommand(socketPath, "oled.sleep-timeout", map[string]any{"seconds": seconds}, "oled sleep-timeout"); err != nil {
		return err
	}

	fmt.Printf("OLED sleep-timeout set to %d seconds\n", seconds)
	return nil
}
