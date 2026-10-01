package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/ipc"
)

// sendCommand sends cmd over the daemon socket and turns a transport failure
// or an !OK response into a single error prefixed with label.
func sendCommand(socketPath, cmd string, args map[string]any, label string) (*ipc.Response, error) {
	resp, err := ipc.Send(socketPath, cmd, args)
	if err != nil {
		return nil, fmt.Errorf("%s: daemon unreachable: %w", label, err)
	}
	if !resp.OK {
		return nil, fmt.Errorf("%s: %s", label, resp.Error)
	}
	return resp, nil
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "pironman",
		Short:        "Control a Pironman 5 case",
		SilenceUsage: true,
	}

	root.AddCommand(newDaemonCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newRGBCmd())
	root.AddCommand(newOLEDCmd())
	root.AddCommand(newFanCmd())
	root.AddCommand(newStatusCmd())

	return root
}
