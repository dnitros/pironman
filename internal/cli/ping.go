package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/ipc"
)

func newPingCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Check that the daemon is reachable over the control socket",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := ipc.Send(ipc.SocketPath(), "ping", nil)
			if err != nil {
				return fmt.Errorf("ping daemon: %w", err)
			}
			if !resp.OK {
				return fmt.Errorf("daemon returned an error: %s", resp.Error)
			}

			fmt.Println("pong")
			return nil
		},
	}
}
