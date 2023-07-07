package baseline

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "Administrative commands to manipulate baseline results.",
	Long: `Administrative commands to manipulate baseline results.
	Baseline results are used to compare the results of the validation tests.
	Those are CI results from reference installations which are used to compare
	the results from custom executions targetting to inference persistent failures,
	helping to isolate:
	- Flaky tests
	- Permanent failures
	- Test environment issues`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			if err := cmd.Help(); err != nil {
				log.Errorf("error loading help(): %v", err)
			}
		}
	},
	Args: cobra.ExactArgs(1),
}

func init() {
	baselineCmd.AddCommand(baselineListCmd)
	baselineCmd.AddCommand(baselineGetCmd)
	baselineCmd.AddCommand(baselineIndexerCmd)
	baselineCmd.AddCommand(baselinePublishCmd)
}

func NewCmdBaseline() *cobra.Command {
	return baselineCmd
}
