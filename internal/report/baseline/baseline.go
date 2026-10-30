// Package baseline holds the baseline report summary data and the functions to
// interact with the results service, backed by CloudFront and S3 storage bucket,
// serving summarized results from CI.
// "Baseline" results are valid/accepted CI executions. The results are processed
// and consumed by OPCT CLI 'report' command to compare the results of the validation
// tests. Those are CI results from reference installations which are used to compare
// the results from custom executions targeting to inference persistent failures,
// helping to isolate:
// - Flaky tests
// - Permanent failures
// - Test environment issues
package baseline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/go-retryablehttp"
	log "github.com/sirupsen/logrus"
)

const (
	bucketNameBaselineReportSummary     = "opct-archive"
	indexObjectKey                      = "api/v0/result/summary/index.json"
	objectPathBaselineReportSummaryPath = "/result/summary/index.json"

	// Path to S3 Object /api/v0/result/summary/{ocpVersion}/{platformType}
	// The S3 is served by S3, which will reduce the costs to access S3, and can be
	// proxies/redirected to other backends without replacing the URL.
	// The original bucket[1], must be migrated to another account and the CloudFront URL,
	// is part of that goal without disrupting the current process.
	// [1] "https://openshift-provider-certification.s3.us-west-2.amazonaws.com"
	cloudfrontDistributionID = "E3MJR7MT6EHHJC"

	// To override those values use environment variables OPCT_EXP_BUCKET_NAME and OPCT_EXP_BUCKET_REGION
	opctStorageBucketName   = "opct-archive"
	opctStorageBucketRegion = "us-east-1"
)

// reportBaseURL is the base URL for the CloudFront-served baseline API.
// It is a package-level variable (not a const) so that tests can override
// it with httptest servers.
var reportBaseURL = "https://d23912a6309zf7.cloudfront.net" //nolint:gochecknoglobals

// setReportBaseURL replaces the base URL used by ReadReportSummaryFromAPI.
// Intended for tests only; returns the old value for deferred restore.
func setReportBaseURL(url string) string {
	old := reportBaseURL
	reportBaseURL = url
	return old
}

// BaselineReport is the struct that holds the baseline report data
// pre-processed and saved in the bucket.
type BaselineConfig struct {
	bucketName               string
	bucketRegion             string
	cloudfrontDistributionID string

	buffer *BaselineData
}

// NewBaselineReportSummary creates a new BaselineConfig struct with the default
// configuration allowing customization to chage the S3 storage used in the management
// tasks.
// TODO deprecate the environment variables when backend is fully migrated to dedicated
// AWS account.
func NewBaselineReportSummary() *BaselineConfig {
	bucketName := opctStorageBucketName
	bucketRegion := opctStorageBucketRegion
	if os.Getenv("OPCT_EXP_BUCKET_NAME") != "" {
		log.Warnf("using custom bucket name: %s", os.Getenv("OPCT_EXP_BUCKET_NAME"))
		bucketName = os.Getenv("OPCT_EXP_BUCKET_NAME")
	}
	if os.Getenv("OPCT_EXP_BUCKET_REGION") != "" {
		log.Warnf("using custom bucket region: %s", os.Getenv("OPCT_EXP_BUCKET_REGION"))
		bucketRegion = os.Getenv("OPCT_EXP_BUCKET_REGION")
	}
	return &BaselineConfig{
		bucketName:               bucketName,
		bucketRegion:             bucketRegion,
		cloudfrontDistributionID: cloudfrontDistributionID,
	}
}

// createS3Clients creates the S3 client and uploader to interact with the S3 storage, checking if
// bucket exists.
//
//nolint:staticcheck
//lint:ignore SA1019 manager kept until transfermanager reaches v1, see createS3Client.
func (brs *BaselineConfig) createS3Clients(ctx context.Context) (*s3.Client, *manager.Uploader, error) {
	if !brs.checkRequiredParams() {
		return nil, nil, fmt.Errorf("missing required parameters or dependencies to enable this feature")
	}

	// create s3 client
	svcS3, uploader, err := createS3Client(ctx, brs.bucketRegion)
	if err != nil {
		return nil, nil, err
	}

	// Check if the bucket exists
	bucketExists, err := checkBucketExists(ctx, svcS3, brs.bucketName)
	if err != nil {
		return nil, nil, err
	}

	if !bucketExists {
		return nil, nil, fmt.Errorf("the OPCT storage does not exists")
	}

	return svcS3, uploader, nil
}

// ReadReportSummaryIndexFromAPI reads the summary report index from the OPCT report URL.
func (brs *BaselineConfig) ReadReportSummaryIndexFromAPI() (*baselineIndex, error) {
	resp, err := brs.ReadReportSummaryFromAPI(objectPathBaselineReportSummaryPath)
	if err != nil {
		log.WithError(err).Error("error reading baseline report summary from API")
		return nil, err
	}
	index := &baselineIndex{}
	err = json.Unmarshal(resp, index)
	if err != nil {
		log.WithError(err).Error("error unmarshalling baseline report summary")
		return nil, err
	}
	return index, nil
}

// ReadReportSummaryFromAPI reads the summary report from the external URL.
func (brs *BaselineConfig) ReadReportSummaryFromAPI(path string) ([]byte, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 5
	retryLogger := log.New()
	retryLogger.SetLevel(log.WarnLevel)
	retryClient.Logger = retryLogger

	url := fmt.Sprintf("%s%s", reportBaseURL, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("X-Custom-Header", "opct")
	req.Header.Set("Content-Type", "application/json")

	client := retryClient.StandardClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	log.Debugf("Summary Report API (%s) response code: %s", path, resp.Status)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("error BaselineAPI request: %s", resp.Status)
	}

	rawResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	return rawResp, nil
}

// GetLatestRawSummaryFromPlatformWithFallback reads the latest summary report from the OPCT report
// service, trying to get the latest summary from the specified platform, and fallback to "None",
// and "AWS", when available.
//
// The workflow parameter scopes the lookup to the matching workflow (e.g.
// "upgrade"). When a workflow is specified every fetched payload is validated
// using InferWorkflow, which checks all available metadata (explicit workflow
// field, provider.runtime.opctConfig run-mode, provider.isUpgradeWorkflow):
//   - A payload whose inferred workflow matches is accepted.
//   - A payload whose inferred workflow differs is rejected.
//   - A payload with no verifiable workflow metadata is skipped — a warning
//     alone does not make an unverifiable workflow match safe.
//
// An empty workflow value accepts the first reachable payload (legacy behavior).
func (brs *BaselineConfig) GetLatestRawSummaryFromPlatformWithFallback(ocpRelease, platformType, workflow string) error {
	var errors []error
	errCount := 0

	// Build candidate paths: workflow-scoped first, then legacy fallback.
	platforms := []string{platformType, "None", "AWS"}
	var evaluatePaths []string
	for _, p := range platforms {
		if workflow != "" {
			evaluatePaths = append(evaluatePaths,
				fmt.Sprintf("/result/summary/%s_%s_%s_latest.json", ocpRelease, p, workflow))
		}
		evaluatePaths = append(evaluatePaths,
			fmt.Sprintf("/result/summary/%s_%s_latest.json", ocpRelease, p))
	}

	for _, path := range evaluatePaths {
		if errCount > (len(evaluatePaths) * 2) {
			log.Errorf("Too many errors, stopping the process")
			break
		}
		body, err := brs.ReadReportSummaryFromAPI(path)
		if err != nil {
			errors = append(errors, fmt.Errorf("error loading BaselineAPI path %s: %v", path, err))
			errCount++
			continue
		}

		bd := &BaselineData{}
		bd.SetRawData(body)

		// When no workflow filter is requested, accept the first successful response.
		if workflow == "" {
			brs.buffer = bd
			return nil
		}

		// Validate the payload's workflow against the requested one using
		// all available metadata (explicit field, run-mode config,
		// isUpgradeWorkflow flag).
		inferredWF, hasEvidence := bd.InferWorkflow()
		if hasEvidence {
			if inferredWF == workflow {
				brs.buffer = bd
				return nil
			}
			// Payload carries a different workflow — reject it.
			log.Debugf("Baseline payload %s has workflow %q, need %q — skipping", path, inferredWF, workflow)
			continue
		}

		// No verifiable workflow metadata — skip this payload entirely.
		// Accepting an unverifiable payload risks cross-workflow
		// contamination (e.g. using a conformance baseline for upgrade).
		log.Debugf("Baseline payload %s has no verifiable workflow metadata — skipping (need %q)", path, workflow)
	}

	if len(errors) > 0 {
		log.Debugf("Found one or more errors while querying baseline API: %v", errors)
	}
	log.Infof("No baseline found matching workflow %q for release %s", workflow, ocpRelease)
	return nil
}

// GetLatestSummaryByPlatform reads the latest summary report from the OPCT report service, trying to
// retrieve from release and platform. When workflow is non-empty the
// workflow-scoped path is tried first. The legacy (workflow-less) path is
// validated with InferWorkflow: a payload whose workflow can be positively
// established and matches is accepted; a mismatch or unverifiable payload
// is rejected. A payload with no verifiable workflow metadata is not
// accepted — unknown provenance cannot safely authorize workflow selection.
// ocpRelease is the OpenShift major version, like "4.7", "4.8", etc.
func (brs *BaselineConfig) GetLatestSummaryByPlatform(ocpRelease, platformType, workflow string) error {
	// Try workflow-scoped path first when a workflow is specified.
	if workflow != "" {
		path := fmt.Sprintf("/result/summary/%s_%s_%s_latest.json", ocpRelease, platformType, workflow)
		buf, err := brs.ReadReportSummaryFromAPI(path)
		if err == nil {
			brs.buffer = &BaselineData{}
			brs.buffer.SetRawData(buf)
			return nil
		}
		log.Debugf("workflow-scoped baseline not found (%s), trying legacy path", path)
	}

	path := fmt.Sprintf("/result/summary/%s_%s_latest.json", ocpRelease, platformType)
	buf, err := brs.ReadReportSummaryFromAPI(path)
	if err != nil {
		return fmt.Errorf("unable to get latest summary by platform: %w", err)
	}

	bd := &BaselineData{}
	bd.SetRawData(buf)

	// If a workflow was requested, validate the fetched payload using
	// all available metadata paths.
	if workflow != "" {
		inferredWF, hasEvidence := bd.InferWorkflow()
		if !hasEvidence {
			log.Warnf("Legacy baseline at %s has no verifiable workflow metadata — skipping (need %q)", path, workflow)
			return nil
		}
		if inferredWF != workflow {
			log.Warnf("Baseline at %s carries workflow %q, need %q — skipping baseline", path, inferredWF, workflow)
			return nil
		}
	}

	brs.buffer = bd
	return nil
}

func (brs *BaselineConfig) GetSummaryByName(name string) ([]byte, error) {
	return brs.ReadReportSummaryFromAPI(fmt.Sprintf("/result/summary/%s.json", name))
}

// checkRequiredParams checks if the required env to enable feature is set, then
// set the default storage names for experimental feature.
func (brs *BaselineConfig) checkRequiredParams() bool {
	log.Debugf("OPCT_ENABLE_ADM_BASELINE=%s", os.Getenv("OPCT_ENABLE_ADM_BASELINE"))
	return os.Getenv("OPCT_ENABLE_ADM_BASELINE") == "1"
}

func (brs *BaselineConfig) GetBuffer() *BaselineData {
	if brs.buffer == nil {
		return nil
	}
	return brs.buffer
}
