package retrieve

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	sonobuoyclient "github.com/vmware-tanzu/sonobuoy/pkg/client"
	config2 "github.com/vmware-tanzu/sonobuoy/pkg/config"
	"golang.org/x/sync/errgroup"

	"github.com/redhat-openshift-ecosystem/opct/internal/cleaner"
	"github.com/redhat-openshift-ecosystem/opct/pkg"
	"github.com/redhat-openshift-ecosystem/opct/pkg/status"
)

func NewCmdRetrieve() *cobra.Command {
	return &cobra.Command{
		Use:   "retrieve",
		Args:  cobra.MaximumNArgs(1),
		Short: "Collect results from validation environment",
		Long:  `Downloads the results archive from the validation environment`,
		RunE: func(cmd *cobra.Command, args []string) error {
			destinationDirectory, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("retrieve finished with errors: %v", err)
			}
			if len(args) == 1 {
				destinationDirectory = args[0]
				finfo, err := os.Stat(destinationDirectory)
				if err != nil {
					return fmt.Errorf("retrieve finished with errors: %v", err)
				}
				if !finfo.IsDir() {
					return fmt.Errorf("retrieve finished with errors: %v", err)
				}
			}

			s := status.NewStatus(&status.StatusInput{Watch: false})
			if err := s.PreRunCheck(); err != nil {
				return fmt.Errorf("retrieve finished with errors: %v", err)
			}

			log.Info("Collecting results...")
			if err := retrieveResultsRetry(s.GetSonobuoyClient(), destinationDirectory); err != nil {
				return fmt.Errorf("retrieve finished with errors: %v", err)
			}

			log.Info("Use the results command to check the validation test summary or share the results archive with your Red Hat partner.")
			return nil
		},
	}
}

func retrieveResultsRetry(sclient sonobuoyclient.Interface, destinationDirectory string) error {
	var err error
	limit := 10 // Retry retrieve 10 times
	pause := time.Second * 2
	retries := 1
	for retries <= limit {
		err = retrieveResults(sclient, destinationDirectory)
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

func retrieveResults(sclient sonobuoyclient.Interface, destinationDirectory string) error {
	// Get a reader that contains the tar output of the results directory.
	reader, ec, err := sclient.RetrieveResults(&sonobuoyclient.RetrieveConfig{
		Namespace: pkg.CertificationNamespace,
		Path:      config2.AggregatorResultsPath,
	})
	if err != nil {
		return errors.Wrap(err, "error retrieving results from sonobuoy")
	}

	// Download results into target directory
	results, err := writeResultsToDirectory(destinationDirectory, reader, ec)
	if err != nil {
		return errors.Wrap(err, "error retrieving results from sonobyuoy")
	}

	// Log the new files to stdout
	for _, result := range results {
		// Rename the file prepending 'opct_' to the name.
		newFile := fmt.Sprintf("%s/opct_%s", filepath.Dir(result), strings.Replace(filepath.Base(result), "sonobuoy_", "", 1))
		log.Debugf("Renaming %s to %s", result, newFile)
		if err := os.Rename(result, newFile); err != nil {
			return fmt.Errorf("error renaming %s to %s: %w", result, newFile, err)
		}
		log.Infof("Results saved to %s", newFile)
	}

	return nil
}

func writeResultsToDirectory(outputDir string, r io.Reader, ec <-chan error) ([]string, error) {
	eg := &errgroup.Group{}
	var results []string
	eg.Go(func() error { return <-ec })
	eg.Go(func() error {
		// scanning for sensitive data
		scannedReader, _, err := cleaner.ScanPatchTarGzipReaderFor(r)
		if err != nil {
			return fmt.Errorf("error scanning results: %w", err)
		}

		// This untars the request itself, which is tar'd as just part of the API request, not the sonobuoy logic.
		filesCreated, err := sonobuoyclient.UntarAll(scannedReader, outputDir, "")
		if err != nil {
			return err
		}
		// Only print the filename if not extracting. Allows capturing the filename for scripting.
		results = append(results, filesCreated...)

		return nil
	})

	return results, eg.Wait()
}
