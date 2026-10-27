package baseline

// TODO move/migrate 'opct exp publish' to this command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/metrics"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/summary"
	"github.com/redhat-openshift-ecosystem/opct/internal/report"
	"github.com/redhat-openshift-ecosystem/opct/internal/report/baseline"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type baselinePublishInput struct {
	forceLatest bool
	verbose     bool
	dryRun      bool
}

var baselinePublishArgs baselinePublishInput
var baselinePublishCmd = &cobra.Command{
	Use:     "publish",
	Example: "opct adm baseline publish <baseline name>",
	Short:   "Publish a baseline result to be used in the review process.",
	Long: `Publish a baseline result to be used in the review process.
	Baseline results are used to compare the results of the validation tests.
	Publishing a baseline result is useful when you want to share the baseline with other users.`,
	Run: baselinePublishCmdRun,
}

func init() {
	baselinePublishCmd.Flags().BoolVarP(
		&baselinePublishArgs.forceLatest, "force-latest", "f", false,
		"Name of the baseline to be published.",
	)
	baselinePublishCmd.Flags().BoolVarP(
		&baselinePublishArgs.verbose, "verbose", "v", false,
		"Show test details of test failures",
	)
	baselinePublishCmd.Flags().BoolVar(
		&baselinePublishArgs.dryRun, "dry-run", false,
		"Process the data and skip publishing the baseline.",
	)
}

func baselinePublishCmdRun(cmd *cobra.Command, args []string) {
	if baselinePublishArgs.forceLatest {
		log.Warn("argument --force-latest <result_name> must be set. Check available baseline with 'opct adm baseline list'")
	}
	// TODOs
	// - check if the baseline exists
	// - read and process as regular 'report' command
	// - check sanity: counts should have acceptable, etc
	// - extract the data to be published, building the name of the file and attributes.
	if len(args) == 0 {
		log.Fatalf("result archive not found: %v", args)
	}
	archive := args[0]
	if _, err := os.Stat(archive); os.IsNotExist(err) {
		log.Fatalf("archive not found: %v", archive)
	}

	fmt.Println()
	log.Infof("Processing baseline result for %s", filepath.Base(archive))

	timers := metrics.NewTimers()
	timers.Add("report-total")

	saveDirectory := "/tmp/opct-tmp-results-" + filepath.Base(archive)
	err := os.Setenv("OPCT_DISABLE_FILTER_BASELINE", "1")
	if err != nil {
		log.Fatalf("error setting variable OPCT_DISABLE_FILTER_BASELINE to skip baseline in the filter pipeline: %v", err)
	}
	cs := summary.NewConsolidatedSummary(&summary.ConsolidatedSummaryInput{
		Verbose: baselinePublishArgs.verbose,
		Timers:  timers,
		Archive: archive,
		SaveTo:  saveDirectory,
	})

	log.Debug("Processing results")
	if err := cs.Process(); err != nil {
		log.Errorf("error processing results: %v", err)
	}

	re := report.NewReportData(false)
	log.Debug("Processing report")
	if err := re.Populate(cs); err != nil {
		log.Errorf("error populating report: %v", err)
	}

	// TODO: ConsolidatedSummary should be migrated to SaveResults
	if err := cs.SaveResults(saveDirectory); err != nil {
		log.Errorf("error saving consolidated summary results: %v", err)
	}
	timers.Add("report-total")
	if err := re.SaveResults(saveDirectory); err != nil {
		log.Errorf("error saving report results: %v", err)
	}

	// CI guardian ensuring baseline results are passing minimaly.
	// TODO: move to config, or allow to add skips.
	// Reject publish when any of those checks failed:
	// https://redhat-openshift-ecosystem.github.io/opct/review/rules/
	// TODO/Validate if need:
	// - OPCT-023*: Test sanity. Enable it when CI pipeline (periodic) is validated
	// - etcd very slow
	if !baselineArchiveHasrequiredChecks(re) {
		log.Fatal("baseline rejected, see the logs for more details.")
	}

	checksStatus := fmt.Sprintf("pass(%d), fail(%d), warn(%d) skip(%d)", len(re.Checks.Pass), len(re.Checks.Fail), len(re.Checks.Warn), len(re.Checks.Skip))
	log.Infof("Baseline checks are OK, proceeding to publish the baseline: %s", checksStatus)

	// Prepare the baseline to publish:
	// - build the metadata from the original report (setup.api)
	// - upload the artifact to /uploads
	// - upload the summary to /api/v0/result/summary
	brs := baseline.NewBaselineReportSummary()
	metaBytes, err := json.Marshal(re.Setup.API)
	if err != nil {
		log.Errorf("error marshaling metadata: %v", err)
	}

	var meta map[string]string
	err = json.Unmarshal(metaBytes, &meta)
	if err != nil {
		log.Errorf("error unmarshalling metadata: %v", err)
	}
	log.Infof("Baseline metadata: %v", meta)
	log.Infof("Uploading baseline to storage")
	// TODO: check if the baseline already exists. It should check the unique
	// id other than the bucket name. The UUID is a good candidate.
	err = brs.UploadBaseline(cmd.Context(), archive, saveDirectory, meta, baselinePublishArgs.dryRun)
	if err != nil {
		log.Fatalf("error uploading baseline: %v", err)
	}

	log.Infof("Success! Baseline result processed from archive: %v", filepath.Base(re.Summary.Tests.Archive))
	log.Infof("You must re-index the storage to serve in the result API. See 'opct adm baseline (indexer|list)'")
}

// baselineArchiveHasrequiredChecks reports whether the processed report is
// eligible to be published as a baseline. A baseline must not carry failures
// of the checks the review process relies on, so OPCT-004, OPCT-005 and
// OPCT-022 in a failed state reject the publication.
//
// It fails closed: a report without evaluated checks cannot be verified, and
// publishing it would seed the review process with an unvetted baseline.
// Every offending check is logged before returning, so a single run reports
// all the reasons a baseline was rejected.
func baselineArchiveHasrequiredChecks(re *report.ReportData) bool {
	if re == nil || re.Checks == nil {
		log.Error("rejecting the baseline: report checks were not evaluated")
		return false
	}
	matchRequiredChecks := true
	for _, check := range re.Checks.Fail {
		if check == nil {
			continue
		}
		if check.ID == report.CheckID004 ||
			check.ID == report.CheckID005 ||
			check.ID == report.CheckID022 {
			errMessage := fmt.Sprintf("%q: want=%q, got=%q", check.SLO, check.SLITarget, check.SLIActual)
			if len(check.Message) > 0 {
				errMessage = fmt.Sprintf("%s: message=%q", errMessage, check.Message)
			}
			log.Errorf("rejecting the baseline, check id %s is in failed state: %s", check.ID, errMessage)
			matchRequiredChecks = false
		}
	}
	return matchRequiredChecks
}
