package baseline

import (
	"os"

	log "github.com/sirupsen/logrus"

	reb "github.com/redhat-openshift-ecosystem/opct/internal/report/baseline"
	"github.com/spf13/cobra"
)

type baselineIndexerInput struct {
	force bool
}

var baselineIndexerArgs baselineIndexerInput
var baselineIndexerCmd = &cobra.Command{
	Use:     "indexer",
	Example: "opct adm baseline indexer",
	Short:   "(Administrative usage) Rebuild the indexer for baseline in the backend.",
	Run:     baselineIndexerCmdRun,
}

func init() {
	baselineListCmd.Flags().BoolVar(&baselineIndexerArgs.force, "force", false, "List all results.")

	// Simple 'check' for non-authorized users, the command will fail later as the user does not have AWS required permissions.
	if baselineIndexerArgs.force && os.Getenv("OPCT_ENABLE_ADM_BASELINE") != "1" {
		log.Fatal("You are not allowed to execute this command.")
	}
}

func baselineIndexerCmdRun(cmd *cobra.Command, args []string) {
	rb := reb.NewBaselineReportSummary()
	err := rb.CreateBaselineIndex()
	if err != nil {
		log.Fatalf("Failed to read index from bucket: %v", err)
	}
	log.Info("Indexer has been updated.")
}
