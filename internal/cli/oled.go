package cli

import (
	"fmt"

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
	resp, err := ipc.Send(socketPath, "oled."+use, nil)
	if err != nil {
		return fmt.Errorf("oled %s: daemon unreachable: %w", use, err)
	}
	if !resp.OK {
		return fmt.Errorf("oled %s: %s", use, resp.Error)
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
	resp, err := ipc.Send(socketPath, "oled.page", map[string]any{"page": page})
	if err != nil {
		return fmt.Errorf("oled page %s: daemon unreachable: %w", page, err)
	}
	if !resp.OK {
		return fmt.Errorf("oled page %s: %s", page, resp.Error)
	}

	fmt.Printf("OLED page set to %s\n", page)
	return nil
}
