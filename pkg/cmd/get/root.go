package get

import (
	"fmt"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get tool information.",
	Run:   runGet,
}

func init() {
	getCmd.AddCommand(imagesCmd)
}

func NewCmdGet() *cobra.Command {
	return getCmd
}

func runGet(cmd *cobra.Command, args []string) {
	fmt.Println("Nothing to do. See -h for more options.")
}
