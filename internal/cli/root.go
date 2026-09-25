// Package cli builds the pironman cobra command tree.
package cli

import "github.com/spf13/cobra"

// NewRootCmd builds the pironman root command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pironman",
		Short: "Control a Pironman 5 case",
	}

	root.AddCommand(newDaemonCmd())
	root.AddCommand(newPingCmd())

	return root
}
