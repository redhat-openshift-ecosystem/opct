package retrieve

import (
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	config2 "github.com/vmware-tanzu/sonobuoy/pkg/config"
	"golang.org/x/sync/errgroup"

	"github.com/openshift/provider-certification-tool/pkg"
	"github.com/openshift/provider-certification-tool/pkg/status"
)

func NewCmdRetrieve(config *pkg.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "retrieve",
		Args:  cobra.MaximumNArgs(1),
		Short: "Collect results from certification environment",
		Long:  `Downloads the results archive from the certification environment`,
		Run: func(cmd *cobra.Command, args []string) {
			destinationDirectory, err := os.Getwd()
			if err != nil {
				log.Error(err)
				return
			}
			if len(args) == 1 {
				destinationDirectory = args[0]
				if _, err := os.Stat(destinationDirectory); err != nil {
					log.Error(err)
					return
				}
			}

			s := status.NewStatusOptions(config)
			err = s.PreRunCheck()
			if err != nil {
				log.Error(err)
				return
			}

			log.Info("Collecting results...")

			if err := retrieveResultsRetry(config, destinationDirectory); err != nil {
				log.Error(err)
				return
			}

			log.Info("Use the 'results' command to check the certification test summary")
		},
	}
}

func retrieveResultsRetry(config *pkg.Config, destinationDirectory string) error {
	var err error
	limit := 10 // Retry retrieve 10 times
	pause := time.Second * 2
	retries := 1
	for retries <= limit {
		err = retrieveResults(config, destinationDirectory)
		if err != nil {
			log.Warn(err)
			if retries+1 < limit {
				log.Warnf("Retrying retrieval %d more times", limit-retries)
			}
			time.Sleep(pause)
			retries++
			continue
		}
		return nil // Retrieved results without a problem
	}

	return errors.New("Retrieval retry limit reached")
}

func retrieveResults(config *pkg.Config, destinationDirectory string) error {
	// Get a reader that contains the tar output of the results directory.
	reader, ec, err := config.SonobuoyClient.RetrieveResults(&client.RetrieveConfig{
		Namespace: pkg.CertificationNamespace,
		Path:      config2.AggregatorResultsPath,
	})
	if err != nil {
		return errors.Wrap(err, "error retrieving results from sonobuoy")
	}

	// Download results into target directory
	err, results := writeResultsToDirectory(destinationDirectory, reader, ec)
	if err != nil {
		return errors.Wrap(err, "error retrieving certification results from sonobyuoy")
	}

	// Log the new files to stdout
	for _, result := range results {
		log.Infof("Results saved to %s", result)
	}

	return nil
}

func writeResultsToDirectory(outputDir string, r io.Reader, ec <-chan error) (error, []string) {
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
