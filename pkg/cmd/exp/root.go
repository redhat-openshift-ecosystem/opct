package exp

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var expCmd = &cobra.Command{
	Use:   "exp",
	Short: "Experimental commands.",
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
	expCmd.AddCommand(cmdPublish)
}

func NewCmdExp() *cobra.Command {
	return expCmd
}
