package cli

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pironman",
		Short: "Control a Pironman 5 case",
	}

	root.AddCommand(newDaemonCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newVersionCmd())

	return root
}
