package cli

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/ipc"
)

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the pironman daemon",
	}
	cmd.AddCommand(newDaemonRunCmd())
	return cmd
}

func newDaemonRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "run",
		Short:  "Run the pironman daemon in the foreground",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDaemon(cmd.Context())
		},
	}
}

func runDaemon(ctx context.Context) error {
	path := ipc.SocketPath()

	srv := ipc.NewServer(map[string]ipc.Handler{
		"ping": handlePing,
	})
	if err := srv.Listen(path); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("pironman daemon listening on %s\n", path)
	return srv.Serve(ctx)
}

func handlePing(args map[string]any) (any, error) {
	return map[string]string{"message": "pong"}, nil
}
