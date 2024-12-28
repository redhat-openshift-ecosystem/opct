package generate

import (
	"github.com/redhat-openshift-ecosystem/opct/internal/report"
	"github.com/spf13/cobra"
)

// NewCommand returns a new cobra.Command for generate docs-checks
func NewCommandDocsChecks() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checks-docs",
		Short: "Generate markdown documentation for checks",
		Long:  "Generate markdown documentation for checks used to build the OPCT documentation.",
	}
	var docsPath string
	cmd.Flags().StringVar(&docsPath, "path", "docs/review/rules.md", "Path to the documentation file")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		csum := report.NewCheckSummary(&report.ReportData{})
		if err := csum.WriteDocumentation(docsPath); err != nil {
			cmd.PrintErrln(err)
			return err
		}
		cmd.Println("File written to:", docsPath)
		return nil
	}

	return cmd
}
