package adm

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var admCmd = &cobra.Command{
	Use:   "adm",
	Short: "Administrative commands.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			err := cmd.Help()
			if err != nil {
				log.Errorf("error loading help(): %v", err)
			}
			os.Exit(0)
		}
	},
}

func init() {
	admCmd.AddCommand(parseMetricsCmd)
	admCmd.AddCommand(parseEtcdLogsCmd)
}

func NewCmdAdm() *cobra.Command {
	return admCmd
}
