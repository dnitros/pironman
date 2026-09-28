package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/ipc"
)

type StatusInfo struct {
	Enabled    bool
	Color      string
	Brightness int
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the daemon's current RGB state",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := gatherStatus(ipc.SocketPath())
			if err != nil {
				return err
			}
			printStatus(info)
			return nil
		},
	}
}

func gatherStatus(socketPath string) (StatusInfo, error) {
	resp, err := ipc.Send(socketPath, "status", nil)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("status: daemon unreachable: %w", err)
	}
	if !resp.OK {
		return StatusInfo{}, fmt.Errorf("status: %s", resp.Error)
	}

	data, _ := resp.Data.(map[string]any)
	enabled, _ := data["enabled"].(bool)
	color, _ := data["color"].(string)
	brightness, _ := data["brightness"].(float64)

	return StatusInfo{Enabled: enabled, Color: color, Brightness: int(brightness)}, nil
}

func printStatus(info StatusInfo) {
	fmt.Printf("RGB enabled: %t\n", info.Enabled)
	fmt.Printf("RGB color: %s\n", info.Color)
	fmt.Printf("RGB brightness: %d\n", info.Brightness)
}
