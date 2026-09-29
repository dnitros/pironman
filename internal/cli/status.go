package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/ipc"
)

type StatusInfo struct {
	Enabled    bool
	Color      string
	Brightness int
	OLEDAwake  bool
	OLEDPage   string
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the daemon's current RGB and OLED state",
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
	oledAwake, _ := data["oled_awake"].(bool)
	oledPage, _ := data["oled_page"].(string)

	return StatusInfo{
		Enabled: enabled, Color: color, Brightness: int(brightness),
		OLEDAwake: oledAwake, OLEDPage: oledPage,
	}, nil
}

func printStatus(info StatusInfo) {
	fmt.Println("RGB")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	fmt.Fprintf(w, "  enabled:\t%t\n", info.Enabled)
	fmt.Fprintf(w, "  color:\t%s\n", info.Color)
	fmt.Fprintf(w, "  brightness:\t%d%%\n", info.Brightness)
	w.Flush()

	fmt.Println("OLED")
	fmt.Fprintf(w, "  awake:\t%t\n", info.OLEDAwake)
	fmt.Fprintf(w, "  page:\t%s\n", info.OLEDPage)
	w.Flush()
}
