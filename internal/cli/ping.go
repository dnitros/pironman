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
			ok, msg := pingDaemon(ipc.SocketPath())
			if !ok {
				return fmt.Errorf("ping daemon: %s", msg)
			}

			fmt.Println("pong")
			return nil
		},
	}
}

func pingDaemon(socketPath string) (ok bool, message string) {
	resp, err := ipc.Send(socketPath, "ping", nil)
	switch {
	case err != nil:
		return false, err.Error()
	case !resp.OK:
		return false, resp.Error
	default:
		return true, ""
	}
}
