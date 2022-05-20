package retrieve

import (
	"io"
	"time"

	"github.com/pkg/errors"
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

			if err := retrieveResultsRetry(config); err != nil {
				log.Error(err)
				return
			}

			log.Info("Use the 'results' command to check the certification test summary")
		},
	}
}

func retrieveResultsRetry(config *pkg.Config) error {
	var err error
	limit := 10 // Retry retrieve 10 times
	pause := time.Second * 2
	retries := 1
	for retries <= limit {
		err = retrieveResults(config)
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

func retrieveResults(config *pkg.Config) error {
	// Get a reader that contains the tar output of the results directory.
	reader, ec, err := config.SonobuoyClient.RetrieveResults(&client.RetrieveConfig{
		Namespace: "sonobuoy",
		Path:      config2.AggregatorResultsPath,
	})
	if err != nil {
		return errors.Wrap(err, "error retrieving results from sonobuoy")
	}

	// Download results into cache directory
	err, results := writeResultsToDirectory(pkg.ResultsDirectory, reader, ec)
	if err != nil {
		return errors.Wrap(err, "error retrieving certification results from sonobyuoy")
	}

	// Log the new files to stdout and save them to a cache for results and upload commands
	for _, result := range results {
		log.Infof("Results saved to %s", result)
	}

	err = results2.SaveToResultsFile(results)
	if err != nil {
		return errors.Wrap(err, "error saving results to cache file")
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
