package results

import (
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/openshift/provider-certification-tool/pkg"
	"github.com/openshift/provider-certification-tool/pkg/status"

	"github.com/vmware-tanzu/sonobuoy/cmd/sonobuoy/app"
)

func NewCmdResults(config *pkg.Config) *cobra.Command {
	return &cobra.Command{
		Use:     "results",
		Aliases: []string{"res"},
		Short:   "Summary of certification results archive",
		Long:    `Generate a readable summary of the results archive retrieved from certification environment`,
		Run: func(cmd *cobra.Command, args []string) {
			s := status.NewStatusOptions(config)
			err := s.PreRunCheck()
			if err != nil {
				log.Error(err)
				return
			}

			resultFiles, err := GetResultFileNames()

			// Validate the results cache file exists
			if err != nil {
				if os.IsNotExist(err) {
					log.Error("The results file was not found")
				} else {
					log.Error(err)
				}
				return
			}

			// Validate the results artifacts exist and use sonobuoy to inspect them
			for _, resultFile := range resultFiles {
				_, err := os.Stat(resultFile)
				if os.IsNotExist(err) {
					log.Errorf("The artifact file %s was not found", resultFile)
					return
				} else if err != nil {
					log.Error(err)
					return
				}

				resultsCmd := app.NewCmdResults()
				resultsCmd.SetArgs([]string{resultFile})

				err = resultsCmd.Execute()
				if err != nil {
					log.Error(err)
					return
				}
			}
		},
	}
}

func SaveToResultsFile(names []string) error {
	output, err := os.Create(path.Join(pkg.ResultsDirectory, pkg.ResultsFileName))
	if err != nil {
		return err
	}
	defer func() {
		err := output.Close()
		if err != nil {
			log.Error(err)
		}
	}()

	for _, n := range names {
		_, err = output.WriteString(n + ";")
		if err != nil {
			return err
		}
	}

	return nil
}

func GetResultFileNames() ([]string, error) {
	var results []string
	contents, err := os.ReadFile(path.Join(pkg.ResultsDirectory, pkg.ResultsFileName))
	if err != nil {
		return results, err
	}
	splitInput := strings.Split(string(contents), ";")
	// Remove trailing empty string
	if len(splitInput) > 1 {
		splitInput = splitInput[:len(splitInput)-1]
	}
	return splitInput, nil
}
