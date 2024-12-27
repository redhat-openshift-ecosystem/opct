package generate

import (
	"github.com/spf13/cobra"
)

// NewCommand returns a new cobra.Command for generate with subcommands
func NewCommandGenerate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate administrative commands",
		Long:  "Generate administrative commands",
	}

	cmd.AddCommand(NewCommandDocsChecks())

	return cmd
}
