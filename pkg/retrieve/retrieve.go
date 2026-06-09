package retrieve

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	sonobuoyclient "github.com/vmware-tanzu/sonobuoy/pkg/client"
	config2 "github.com/vmware-tanzu/sonobuoy/pkg/config"
	pluginaggregation "github.com/vmware-tanzu/sonobuoy/pkg/plugin/aggregation"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/redhat-openshift-ecosystem/opct/internal/cleaner"
	"github.com/redhat-openshift-ecosystem/opct/pkg"
	opclient "github.com/redhat-openshift-ecosystem/opct/pkg/client"
	"github.com/redhat-openshift-ecosystem/opct/pkg/status"
)

func NewCmdRetrieve() *cobra.Command {
	var skipRedact bool

	cmd := &cobra.Command{
		Use:   "retrieve",
		Args:  cobra.MaximumNArgs(1),
		Short: "Collect results from validation environment",
		Long:  `Downloads the results archive from the validation environment`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if skipRedact {
				log.Warn("═════════════════════════════════════════════════════════════")
				log.Warn("WARNING: --debug-only-skip-redact is enabled")
				log.Warn("WARNING: Sensitive data will NOT be redacted from the archive")
				log.Warn("WARNING: DO NOT share this archive externally")
				log.Warn("WARNING: Archive may contain credentials, tokens, and secrets")
				log.Warn("═════════════════════════════════════════════════════════════")
				cleaner.SkipRedaction = true
			}

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
			if err := retrieveResultsRetry(destinationDirectory); err != nil {
				return fmt.Errorf("retrieve finished with errors: %v", err)
			}

			log.Info("Run 'opct report -s ./report <archive>.tar.gz' to review the validation results.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&skipRedact, "debug-only-skip-redact", false,
		"Skip redaction of sensitive data (DEBUG ONLY - NOT RECOMMENDED)")
	_ = cmd.Flags().MarkHidden("debug-only-skip-redact")

	return cmd
}

func retrieveResultsRetry(destinationDirectory string) error {
	var err error
	limit := 10
	pause := time.Second * 2
	retries := 1
	for retries <= limit {
		err = retrieveResults(destinationDirectory)
		if err != nil {
			log.Error(err)
			if retries+1 < limit {
				log.Warnf("Retrying retrieval %d more times after %d sec", limit-retries, pause/time.Second)
			}
			time.Sleep(pause)
			retries++
			continue
		}
		return nil
	}

	return fmt.Errorf("retrieval retry limit reached")
}

func retrieveResults(destinationDirectory string) error {
	// Phase 1: Download archive to temp file
	tmpFile, err := downloadFromPod()
	if err != nil {
		return fmt.Errorf("error retrieving results from aggregator server: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile) }()

	// Phase 2: Scan/redact from disk
	log.Info("Scanning archive for sensitive data...")
	fin, err := os.Open(tmpFile)
	if err != nil {
		return fmt.Errorf("error opening downloaded archive: %w", err)
	}
	defer func() { _ = fin.Close() }()

	scannedReader, _, err := cleaner.ScanPatchTarGzipReaderFor(fin)
	if err != nil {
		return fmt.Errorf("error scanning results: %w", err)
	}

	// Phase 3: Save scanned archive and extract
	scannedFile, err := os.CreateTemp("", "opct-scanned-*.tar.gz")
	if err != nil {
		return fmt.Errorf("error creating temp file for scanned archive: %w", err)
	}
	scannedPath := scannedFile.Name()
	defer func() { _ = os.Remove(scannedPath) }()

	if _, err := io.Copy(scannedFile, scannedReader); err != nil {
		_ = scannedFile.Close()
		return fmt.Errorf("error writing scanned archive: %w", err)
	}
	if err := scannedFile.Close(); err != nil {
		return fmt.Errorf("error closing scanned archive: %w", err)
	}

	// Reopen for extraction
	scannedIn, err := os.Open(scannedPath)
	if err != nil {
		return fmt.Errorf("error reopening scanned archive: %w", err)
	}
	defer func() { _ = scannedIn.Close() }()

	filesCreated, err := sonobuoyclient.UntarAll(scannedIn, destinationDirectory, "")
	if err != nil {
		return fmt.Errorf("error extracting results: %w", err)
	}

	for _, result := range filesCreated {
		newFile := fmt.Sprintf("%s/opct_%s", filepath.Dir(result), strings.Replace(filepath.Base(result), "sonobuoy_", "", 1))
		log.Debugf("Renaming %s to %s", result, newFile)
		if err := os.Rename(result, newFile); err != nil {
			return fmt.Errorf("error renaming %s to %s: %w", result, newFile, err)
		}
		log.Infof("Results saved to %s", newFile)
	}

	return nil
}

// downloadFromPod downloads the results archive from the sonobuoy aggregator pod
// to a temp file using WebSocket (with SPDY fallback).
func downloadFromPod() (string, error) {
	cli, err := opclient.NewClient()
	if err != nil {
		return "", fmt.Errorf("error creating kubernetes client: %w", err)
	}

	podName, err := pluginaggregation.GetAggregatorPodName(cli.KClient, pkg.CertificationNamespace)
	if err != nil {
		return "", fmt.Errorf("failed to get aggregator server's pod: %w", err)
	}

	restClient := cli.KClient.CoreV1().RESTClient()
	req := restClient.Post().
		Resource("pods").
		Name(podName).
		Namespace(pkg.CertificationNamespace).
		SubResource("exec").
		Param("container", config2.AggregatorContainerName)
	req.VersionedParams(&corev1.PodExecOptions{
		Container: config2.AggregatorContainerName,
		Command:   []string{"/sonobuoy", "splat", config2.AggregatorResultsPath},
		Stdin:     false,
		Stdout:    true,
		Stderr:    false,
	}, scheme.ParameterCodec)

	// WebSocket primary, SPDY fallback
	wsExec, err := remotecommand.NewWebSocketExecutor(cli.RestConfig, "POST", req.URL().String())
	if err != nil {
		return "", fmt.Errorf("error creating WebSocket executor: %w", err)
	}
	spdyExec, err := remotecommand.NewSPDYExecutor(cli.RestConfig, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("error creating SPDY executor: %w", err)
	}
	exec, err := remotecommand.NewFallbackExecutor(wsExec, spdyExec, httpstream.IsUpgradeFailure)
	if err != nil {
		return "", fmt.Errorf("error creating fallback executor: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "opct-retrieve-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %w", err)
	}

	log.Infof("Downloading archive from aggregator server...")
	log.Debugf("Discovered aggregator server running on pod %s/%s...", pkg.CertificationNamespace, podName)
	startTime := time.Now()

	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: tmpFile,
		Tty:    false,
	})
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("error streaming results from pod: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("error closing temp file: %w", err)
	}

	fi, err := os.Stat(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("error stat temp file: %w", err)
	}
	log.Infof("Downloaded %.1f MB in %s", float64(fi.Size())/(1024*1024), time.Since(startTime).Round(time.Second))

	return tmpFile.Name(), nil
}
