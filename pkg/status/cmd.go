package status

import (
	"github.com/redhat-openshift-ecosystem/opct/pkg/wait"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// cmdStatusArgs is the struct to store the input arguments for the status command.
type cmdInputStatus struct {
	Watch           bool
	IntervalSeconds int
}

var cmdArgsStatus cmdInputStatus
var cmdStatus = &cobra.Command{
	Use:     "status",
	Example: "opct status --watch",
	Short:   "Show the current status of the validation tool",
	Long:    ``,
	RunE:    cmdStatusRun,
}

func init() {
	cmdStatus.PersistentFlags().BoolVarP(&cmdArgsStatus.Watch, "watch", "w", false, "Keep watch status after running")
	cmdStatus.Flags().IntVarP(&cmdArgsStatus.IntervalSeconds, "watch-interval", "", DefaultStatusIntervalSeconds, "Interval to watch the status and print in the stdout")
}

func NewCmdStatus() *cobra.Command {
	return cmdStatus
}

func cmdStatusRun(cmd *cobra.Command, args []string) error {
	o := NewStatus(&StatusInput{
		Watch:           cmdArgsStatus.Watch,
		IntervalSeconds: cmdArgsStatus.IntervalSeconds,
	})
	// Pre-checks and setup
	if err := o.PreRunCheck(); err != nil {
		log.WithError(err).Error("error running pre-checks")
		return err
	}

	// Wait for Sonobuoy to create
	if err := wait.WaitForRequiredResources(o.kclient); err != nil {
		log.WithError(err).Error("error waiting for sonobuoy pods to become ready")
		return err
	}

	// Wait for Sononbuoy to start reporting status
	if err := o.WaitForStatusReport(cmd.Context()); err != nil {
		log.WithError(err).Error("error retrieving current aggregator status")
		return err
	}

	if err := o.Print(cmd); err != nil {
		log.WithError(err).Error("error printing status")
		return err
	}
	return nil
}
