package retrieve

import (
	"io"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	config2 "github.com/vmware-tanzu/sonobuoy/pkg/config"
	"golang.org/x/sync/errgroup"

	"github.com/openshift/provider-certification-tool/pkg"
	results2 "github.com/openshift/provider-certification-tool/pkg/results"
	"github.com/openshift/provider-certification-tool/pkg/status"
)

func NewCmdRetrieve(config *pkg.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "retrieve",
		Short: "Collect results from certification environment",
		Long:  `Downloads the results archive from the certification environment`,
		Run: func(cmd *cobra.Command, args []string) {
			s := status.NewStatusOptions(config)
			err := s.PreRunCheck()
			if err != nil {
				log.Error(err)
				return
			}

			log.Info("Collecting results...")

			// Get a reader that contains the tar output of the results directory.
			reader, ec, err := config.SonobuoyClient.RetrieveResults(&client.RetrieveConfig{
				Namespace: "sonobuoy",
				Path:      config2.AggregatorResultsPath,
			})
			if err != nil {
				log.WithError(err).Error("error retrieving results from sonobuoy")
				return
			}

			// Untar the request into current directory
			err, results := retrieveResults(pkg.ResultsDirectory, reader, ec)
			if err != nil {
				log.WithError(err).Error("error retrieving certification results from sonobyuoy")
				return
			}

			// Log the new files to stdout and save them to a cache for results and upload commands
			for _, result := range results {
				log.Infof("Results saved to %s", result)
			}

			err = results2.SaveToResultsFile(results)
			if err != nil {
				log.WithError(err).Error("error saving results to cache file")
			}

			log.Info("Use the 'results' command to check the certification test summary")
		},
	}
}

func retrieveResults(outputDir string, r io.Reader, ec <-chan error) (error, []string) {
	eg := &errgroup.Group{}
	var results []string
	eg.Go(func() error { return <-ec })
	eg.Go(func() error {
		// This untars the request itself, which is tar'd as just part of the API request, not the sonobuoy logic.
		filesCreated, err := client.UntarAll(r, outputDir, "")
		if err != nil {
			return err
		}
		// Only print the filename if not extracting. Allows capturing the filename for scripting.
		for _, name := range filesCreated {
			results = append(results, name)
		}

		return nil
	})

	return eg.Wait(), results
}

/**
func NewCmdRetrieve(config *runtime.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "retrieve",
		Short: "Collect results from Certification environment",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("Collecting results...")

			// Setup Sonobuoy command call
			retrieveCmd := app.NewCmdRetrieve()

			// Configure the flags
			retrieveCmd.Flags().Set("kubeconfig", config.Kubeconfig)

			// Retry the retrieve command
			retries := 1
			err := wait2.PollImmediateUntilWithContext(context.TODO(), status.StatusInterval, func(ctx context.Context) (done bool, err error) {
				if retries == status.StatusRetryLimit {
					return true, errors.New("retry limit reached trying to retrieve results")
				}

				err = retrieveCmd.Execute()
				if err != nil {
					retries++
					return false, err
				}

				return true, nil
			})

			if err != nil {
				log.WithError(err).Error("error retrieving certification results")
			}
		},
	}
}
*/
