package e2ed

import (
	"github.com/spf13/cobra"
)

// NewCommand returns a new cobra.Command for e2e-dedicated with subcommands
func NewCommandE2eDedicated() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "e2e-dedicated",
		Short: "Administrative commands for e2e-dedicated node configuration",
	}

	cmd.AddCommand(taintNodeCmd)

	return cmd
}
