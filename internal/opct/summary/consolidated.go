// Package summary provides the entrypoint to process the results of the provider and baseline
// validations, applying filters and transformations to the data.
package summary

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/metrics"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/redhat-openshift-ecosystem/opct/internal/openshift/ci/sippy"
	"github.com/redhat-openshift-ecosystem/opct/internal/report/baseline"
)

// ConsolidatedSummary Aggregate the results of provider and baseline
type ConsolidatedSummary struct {
	Verbose     bool
	Timers      *metrics.Timers
	Provider    *ResultSummary
	Baseline    *ResultSummary
	BaselineAPI *baseline.BaselineConfig
}

type ConsolidatedSummaryInput struct {
	Archive     string
	ArchiveBase string
	SaveTo      string
	Verbose     bool
	Timers      *metrics.Timers
}

func NewConsolidatedSummary(in *ConsolidatedSummaryInput) *ConsolidatedSummary {
	return &ConsolidatedSummary{
		Verbose: in.Verbose,
		Timers:  in.Timers,
		Provider: &ResultSummary{
			Name:      ResultSourceNameProvider,
			Archive:   in.Archive,
			OpenShift: &OpenShiftSummary{},
			Sonobuoy:  NewSonobuoySummary(),
			Suites: &OpenshiftTestsSuites{
				OpenshiftConformance:  &OpenshiftTestsSuite{Name: "openshiftConformance"},
				KubernetesConformance: &OpenshiftTestsSuite{Name: "kubernetesConformance"},
			},
			SavePath: in.SaveTo,
		},
		Baseline: &ResultSummary{
			Name:      ResultSourceNameBaseline,
			Archive:   in.ArchiveBase,
			OpenShift: &OpenShiftSummary{},
			Sonobuoy:  NewSonobuoySummary(),
			Suites: &OpenshiftTestsSuites{
				OpenshiftConformance:  &OpenshiftTestsSuite{Name: "openshiftConformance"},
				KubernetesConformance: &OpenshiftTestsSuite{Name: "kubernetesConformance"},
			},
		},
		BaselineAPI: &baseline.BaselineConfig{},
	}
}

// Process entrypoint to read and fill all summaries for each archive, plugin and suites
// applying any transformation it needs through filters.
func (cs *ConsolidatedSummary) Process() error {
	cs.Timers.Add("cs-process")

	// Load Result Summary from Archives
	log.Debug("Processing results/Populating Provider")
	cs.Timers.Set("cs-process/populate-provider")
	if err := cs.Provider.Populate(); err != nil {
		return fmt.Errorf("processing provider results: %w", err)
	}

	log.Debug("Processing results/Populating Baseline")
	cs.Timers.Set("cs-process/populate-baseline")
	if err := cs.Baseline.Populate(); err != nil {
		return fmt.Errorf("processing baseline results: %w", err)
	}

	// Filters pipeline (order matters)
	log.Debug("Processing results/Applying filters/Suite")
	cs.Timers.Set("cs-process/filter1-suite")
	if err := cs.applyFilterSuite(); err != nil {
		return err
	}

	log.Debug("Processing results/Applying filters/Known Failures")
	cs.Timers.Set("cs-process/filter5-known-failures")
	if err := cs.applyFilterKnownFailures(plugin.FilterNameKF); err != nil {
		return err
	}

	log.Debug("Processing results/Applying filters/6/Replay")
	cs.Timers.Set("cs-process/filter-replay")
	if err := cs.applyFilterReplay(plugin.FilterNameReplay); err != nil {
		return err
	}

	log.Debug("Processing results/Applying filters/2/Baseline")
	cs.Timers.Set("cs-process/filter2-baseline")
	if err := cs.applyFilterBaseline(plugin.FilterNameBaseline); err != nil {
		return err
	}

	log.Debug("Processing results/Applying filters/3/Flake")
	cs.Timers.Set("cs-process/filter3-flake")
	if err := cs.applyFilterFlaky(plugin.FilterNameFlaky); err != nil {
		return err
	}

	log.Debug("Processing results/Applying filters/4/Baseline API")
	cs.Timers.Set("cs-process/filter4-baseline-api")
	if err := cs.applyFilterBaselineAPI(); err != nil {
		return err
	}

	log.Debug("Processing results/Applying filters/Saving final filter")
	cs.Timers.Set("cs-process/filter-finish")
	if err := cs.applyFilterCopyPipeline(plugin.FilterNameFinalCopy); err != nil {
		return err
	}

	// Build documentation for failures.
	log.Debug("Processing results/Building tests documentation")
	cs.Timers.Set("cs-process/build-docs")
	if err := cs.buildDocumentation(); err != nil {
		return err
	}

	cs.Timers.Add("cs-process")
	return nil
}

// GetProvider get the provider results.
func (cs *ConsolidatedSummary) GetProvider() *ResultSummary {
	return cs.Provider
}

// GetBaseline get the baseline results.
func (cs *ConsolidatedSummary) GetBaseline() *ResultSummary {
	return cs.Baseline
}

// HasBaselineResults checks if the baseline results was set (--dif),
// and has valid data.
func (cs *ConsolidatedSummary) HasBaselineResults() bool {
	if cs.Baseline == nil {
		return false
	}
	return cs.Baseline.HasValidResults()
}

// Filter1: Suite
// applyFilterSuite process the FailedList for each plugin, getting **intersection** tests
// for respective suite.
func (cs *ConsolidatedSummary) applyFilterSuite() error {
	for _, pluginName := range []string{
		plugin.PluginNameOpenShiftUpgrade,
		plugin.PluginNameKubernetesConformance,
		plugin.PluginNameOpenShiftConformance,
		plugin.PluginNameConformanceReplay,
	} {
		if err := cs.applyFilterSuiteForPlugin(pluginName); err != nil {
			return fmt.Errorf("error while processing filter1 (SuiteOnly): %w", err)
		}
	}
	return nil
}

// applyFilterSuiteForPlugin calculates the intersection of Provider Failed AND suite
func (cs *ConsolidatedSummary) applyFilterSuiteForPlugin(pluginName string) error {
	var ps *plugin.OPCTPluginSummary
	var pluginSuite *OpenshiftTestsSuite

	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultK8SValidated()
		pluginSuite = cs.GetProvider().GetSuites().KubernetesConformance
	case plugin.PluginNameOpenShiftConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultOCPValidated()
		pluginSuite = cs.GetProvider().GetSuites().OpenshiftConformance

	case plugin.PluginNameOpenShiftUpgrade:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceUpgrade()
		pluginSuite = &OpenshiftTestsSuite{}

	case plugin.PluginNameConformanceReplay:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceReplay()
		pluginSuite = &OpenshiftTestsSuite{}
	}

	e2eFailures := ps.FailedList
	e2eSuite := pluginSuite.Tests
	emptySuite := len(pluginSuite.Tests) == 0
	hashSuite := make(map[string]struct{}, len(e2eSuite))

	for _, v := range e2eSuite {
		hashSuite[v] = struct{}{}
	}

	for _, v := range e2eFailures {
		// move on the pipeline when the suite is empty.
		ps.Tests[v].State = "filter1SuiteOnly"

		// Skip when the suite has no tests or issues when collecting the counter.
		if emptySuite {
			ps.FailedFilter1 = append(ps.FailedFilter1, v)
			continue
		}
		// save the test in suite, and excluded ones.
		if _, ok := hashSuite[v]; ok {
			ps.FailedFilter1 = append(ps.FailedFilter1, v)
			continue
		}
		ps.FailedExcludedFilter1 = append(ps.FailedExcludedFilter1, v)
	}
	sort.Strings(ps.FailedFilter1)

	log.Debugf("Filter (SuiteOnly) results: plugin=%s in=failures(%d) in=suite(%d) out=filter(%d) filterExcluded(%d)",
		pluginName, len(e2eFailures), len(e2eSuite),
		len(ps.FailedFilter1), len(ps.FailedExcludedFilter1))
	return nil
}

// Filter2: Baseline archive
// applyFilterBaseline process the FailedFilterSuite for each plugin, **excluding** failures from
// baseline test.
func (cs *ConsolidatedSummary) applyFilterBaseline(filterID string) error {
	for _, pluginName := range []string{
		plugin.PluginNameOpenShiftUpgrade,
		plugin.PluginNameKubernetesConformance,
		plugin.PluginNameOpenShiftConformance,
		plugin.PluginNameConformanceReplay,
	} {
		if err := cs.applyFilterBaselineForPlugin(pluginName, filterID); err != nil {
			return fmt.Errorf("error while processing filter2 (baseline archive): %w", err)
		}
	}
	return nil
}

// applyFilterBaselineForPlugin calculates the **exclusion** tests of
// Provider Failed included on suite and Baseline failed tests.
func (cs *ConsolidatedSummary) applyFilterBaselineForPlugin(pluginName string, filterID string) error {
	var ps *plugin.OPCTPluginSummary
	var e2eFailuresBaseline []string

	// TODO: replace the baseline from discovered data from API (s3). The flag
	// OPCT_DISABLE_EXP_BASELINE_API can be set to use the local file.
	// Default method is to use the API to get the baseline.

	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultK8SValidated()
		if cs.GetBaseline().HasValidResults() {
			e2eFailuresBaseline = cs.GetBaseline().GetOpenShift().GetResultK8SValidated().FailedList
		}
	case plugin.PluginNameOpenShiftConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultOCPValidated()
		if cs.GetBaseline().HasValidResults() {
			e2eFailuresBaseline = cs.GetBaseline().GetOpenShift().GetResultOCPValidated().FailedList
		}

	case plugin.PluginNameOpenShiftUpgrade:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceUpgrade()

	case plugin.PluginNameConformanceReplay:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceReplay()

	default:
		return errors.New("Suite not found to apply filter: Flaky")
	}

	filterFailures, filterFailuresExcluded := ps.GetFailuresByFilterID(filterID)
	e2eFailuresProvider := ps.GetPreviousFailuresByFilterID(filterID)
	hashBaseline := make(map[string]struct{}, len(e2eFailuresBaseline))

	for _, v := range e2eFailuresBaseline {
		hashBaseline[v] = struct{}{}
	}

	// DEPRECATION warning when used:
	if len(e2eFailuresBaseline) > 0 {
		log.Warnf("Filter baseline (--diff|--baseline) is deprecated and will be removed soon, the filter BaselineAPI is replacing and automatically applied to the failure pipeline.")
	}
	for _, v := range e2eFailuresProvider {
		ps.Tests[v].State = "filter2Baseline"
		if _, ok := hashBaseline[v]; !ok {
			filterFailures = append(filterFailures, v)
			continue
		}
		filterFailuresExcluded = append(filterFailuresExcluded, v)
	}
	sort.Strings(filterFailures)
	ps.SetFailuresByFilterID(filterID, filterFailures, filterFailuresExcluded)

	log.Debugf("Filter (Baseline) results: plugin=%s in=filter(%d) out=filter(%d) filterExcluded(%d)",
		pluginName, len(e2eFailuresProvider),
		len(filterFailures), len(filterFailuresExcluded))
	return nil
}

// Filter3: Flaky
// applyFilterFlaky process the FailedFilterSuite for each plugin, **excluding** failures from
// baseline test.
func (cs *ConsolidatedSummary) applyFilterFlaky(filterID string) error {
	if err := cs.applyFilterFlakeForPlugin(plugin.PluginNameKubernetesConformance, filterID); err != nil {
		return err
	}
	if err := cs.applyFilterFlakeForPlugin(plugin.PluginNameOpenShiftConformance, filterID); err != nil {
		return err
	}
	return nil
}

// applyFilterFlakeForPlugin query the Sippy API looking for each failed test
// on each plugin/suite, saving the list on the ResultSummary.
func (cs *ConsolidatedSummary) applyFilterFlakeForPlugin(pluginName string, filterID string) error {
	var ps *plugin.OPCTPluginSummary

	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultK8SValidated()

	case plugin.PluginNameOpenShiftConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultOCPValidated()

	case plugin.PluginNameOpenShiftUpgrade:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceUpgrade()

	case plugin.PluginNameConformanceReplay:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceReplay()

	default:
		return errors.New("Suite not found to apply filter: Flaky")
	}

	// TODO: define if we will check for flakes for all failures or only filtered
	// Query Flaky only the FilteredBaseline to avoid many external queries.
	ver, err := cs.GetProvider().GetOpenShift().GetClusterVersionXY()
	if err != nil {
		return errors.Errorf("Error getting cluster version: %v", err)
	}

	api := sippy.NewSippyAPI(ver)
	for _, name := range ps.FailedFilter2 {
		ps.Tests[name].State = "filter3FlakeCheck"
		resp, err := api.QueryTests(&sippy.SippyTestsRequestInput{TestName: name})
		if err != nil {
			log.Errorf("#> Error querying to Sippy API: %v", err)
			ps.FailedFilter3 = append(ps.FailedFilter3, name)
			continue
		}
		if resp == nil {
			log.Errorf("Error filter flakeAPI: invalid response: %v", resp)
			ps.FailedFilter3 = append(ps.FailedFilter3, name)
			continue
		}
		for _, r := range *resp {
			if _, ok := ps.Tests[name]; ok {
				ps.Tests[name].Flake = &r
			} else {
				ps.Tests[name] = &plugin.TestItem{
					Name:  name,
					Flake: &r,
				}
			}
			// Applying flake filter by moving only non-flakes to the pipeline.
			// The tests reporing lower than 5% of CurrentFlakePerc by Sippy are selected as non-flake.
			// TODO: Review flake severity
			if ps.Tests[name].Flake.CurrentFlakePerc <= 5.0 {
				ps.Tests[name].State = "filter3Priority"
				ps.FailedFilter3 = append(ps.FailedFilter3, name)
				continue
			}
			ps.Tests[name].State = "filter3Flake"
			ps.FailedExcludedFilter3 = append(ps.FailedExcludedFilter3, name)
		}
	}
	sort.Strings(ps.FailedFilter3)

	log.Debugf("Filter (FlakeAPI) results: plugin=%s in=filter(%d) out=filter(%d) filterExcluded(%d)",
		pluginName, len(ps.FailedFilter2),
		len(ps.FailedFilter3), len(ps.FailedExcludedFilter3))
	return nil
}

// Filter4: Baseline API
func (cs *ConsolidatedSummary) applyFilterBaselineAPI() error {
	// Load baseline results from API
	if err := cs.loadBaselineFromAPI(); err != nil {
		return fmt.Errorf("loading baseline results from API: %w", err)
	}
	for _, pluginName := range []string{
		plugin.PluginNameOpenShiftUpgrade,
		plugin.PluginNameKubernetesConformance,
		plugin.PluginNameOpenShiftConformance,
		plugin.PluginNameConformanceReplay,
	} {
		if err := cs.applyFilterBaselineAPIForPlugin(pluginName); err != nil {
			return fmt.Errorf("error while processing filter4 (baseline API): %w", err)
		}
	}
	return nil
}

// loadBaselineFromAPI query the the OPCT "backend" looking for the baseline results.
func (cs *ConsolidatedSummary) loadBaselineFromAPI() error {
	if os.Getenv("OPCT_DISABLE_FILTER_BASELINE") == "1" {
		log.Warnf("Filter pipeline: Basline API is explicitly disabled by OPCT_DISABLE_FILTER_BASELINE, skipping the discoverying baseline results from API")
		return nil
	}
	// Path to S3 Object /api/v0/result/summary/{ocpVersion}/{platformType}
	// The S3 is served by S3, which will reduce the costs to access S3, and can be
	// proxies/redirected to other backends without replacing the URL.
	// The original bucket[1], must be migrated to another account and the CloudFront URL,
	// is part of that goal without disrupting the current process.
	// [1] "https://openshift-provider-certification.s3.us-west-2.amazonaws.com"
	// baseURL := "https://d23912a6309zf7.cloudfront.net/api/v0"

	// Result to evaluate before returning failure
	ocpRelease, err := cs.Provider.OpenShift.GetClusterVersionXY()
	if err != nil {
		os, err := cs.Provider.OpenShift.GetClusterVersion()
		if err != nil {
			return errors.Errorf("Error getting cluster version: %v", err)
		}
		ocpRelease = fmt.Sprintf("%s.%s", strings.Split(os.Desired, ".")[0], strings.Split(os.Desired, ".")[1])
	}
	platformType := cs.Provider.OpenShift.GetInfrastructurePlatformType()

	cs.BaselineAPI = baseline.NewBaselineReportSummary()
	if err := cs.BaselineAPI.GetLatestRawSummaryFromPlatformWithFallback(ocpRelease, platformType); err != nil {
		return errors.Wrap(err, "failed to get baseline from API")
	}
	return nil
}

// applyFilterBaselineAPIForPlugin check the Sippy API looking for each failed test
// on each plugin/suite, saving the list on the ResultSummary.
// The filter must populate the FailedFilter4 and FailedExcludedFilter4.
func (cs *ConsolidatedSummary) applyFilterBaselineAPIForPlugin(pluginName string) error {
	// log.Warnf("TODO: implement applyFilterBaselineAPIForPlugin: %s", pluginName)
	var ps *plugin.OPCTPluginSummary
	var e2eFailuresBaseline []string
	var err error

	// TODO: replace the baseline from discovered data from API (s3). The flag
	// OPCT_DISABLE_EXP_BASELINE_API can be set to use the local file.
	// Default method is to use the API to get the baseline.

	skipFilter := false
	if os.Getenv("OPCT_DISABLE_FILTER_BASELINE") == "1" {
		skipFilter = true
	}

	doneFilter := func() {
		log.Debugf("Filter (BaselineAPI) results: plugin=%s in=filter(%d) inApi=(%d) out=filter(%d) excluded(%d)",
			pluginName, len(ps.FailedFilter3), len(e2eFailuresBaseline),
			len(ps.FailedFilter4), len(ps.FailedExcludedFilter4))
	}

	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultK8SValidated()

	case plugin.PluginNameOpenShiftConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultOCPValidated()

	case plugin.PluginNameOpenShiftUpgrade:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceUpgrade()

	case plugin.PluginNameConformanceReplay:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceReplay()
		ps.FailedFilter4 = ps.FailedFilter3
		doneFilter()
		return nil

	default:
		return fmt.Errorf("plugin not found")
	}

	b := cs.BaselineAPI.GetBuffer()
	if b != nil {
		e2eFailuresBaseline, err = b.GetPriorityFailuresFromPlugin(pluginName)
		if err != nil {
			log.Errorf("failed to get priority failures from plugin: %v", err)
		}
	}

	e2eFailuresPipeline := ps.FailedFilter3
	hashBaseline := make(map[string]struct{}, len(e2eFailuresPipeline))

	for _, v := range e2eFailuresBaseline {
		hashBaseline[v] = struct{}{}
	}

	for _, v := range e2eFailuresPipeline {
		ps.Tests[v].State = "filter4BaselineAPI"
		if _, ok := hashBaseline[v]; !ok {
			ps.FailedFilter4 = append(ps.FailedFilter4, v)
			continue
		}
		ps.FailedExcludedFilter4 = append(ps.FailedExcludedFilter4, v)
	}

	// feed the pipeline with the same tests when the filter is disabled.
	if skipFilter {
		log.Warn("Filter pipeline: Basline API is explicitly disabled by OPCT_DISABLE_FILTER_BASELINE, using Filter3 to keep processing failures")
		ps.FailedFilter4 = ps.FailedFilter3
	}
	sort.Strings(ps.FailedFilter4)
	doneFilter()
	return nil
}

// Filter5: Known Failures
// applyFilterKnownFailures skip well known failures that are not relevant to the validation process.
func (cs *ConsolidatedSummary) applyFilterKnownFailures(filterID string) error {
	// Reason to skip the test:
	// "[sig-arch] External binary usage" :
	//  - The test is not relevant to the validation process, and it's not a real failure
	//    since the k8s/conformance suite is executed correctly.
	// "[sig-mco] Machine config pools complete upgrade" :
	//  - The test is not relevant to the validation process, the custom MCP is used
	//    in the OPCT topology to executed in-cluster validation. If MCP is not used,
	//    the test environment would be evicted when the dedicated node is drained.
	cs.Provider.TestSuiteKnownFailures = []string{
		"[sig-arch] External binary usage",
		"[sig-mco] Machine config pools complete upgrade",
	}

	for _, pluginName := range []string{
		plugin.PluginNameOpenShiftUpgrade,
		plugin.PluginNameKubernetesConformance,
		plugin.PluginNameOpenShiftConformance,
		plugin.PluginNameConformanceReplay,
	} {
		if err := cs.applyFilterKnownFailuresForPlugin(pluginName, filterID); err != nil {
			return fmt.Errorf("error while processing filter5 (baseline API): %w", err)
		}
	}
	return nil
}

// Filter5 by plugin
func (cs *ConsolidatedSummary) applyFilterKnownFailuresForPlugin(pluginName string, filterID string) error {
	var ps *plugin.OPCTPluginSummary

	// Get the list of the last filter in the pipeline
	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultK8SValidated()

	case plugin.PluginNameOpenShiftConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultOCPValidated()

	case plugin.PluginNameOpenShiftUpgrade:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceUpgrade()

	case plugin.PluginNameConformanceReplay:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceReplay()

	default:
		return fmt.Errorf("error while processing filter5 (know failures), plugin not found: %s", pluginName)
	}

	// read the failures from pipeline
	filterFailures, filterFailuresExcluded := ps.GetFailuresByFilterID(filterID)
	e2eFailuresPipeline := ps.GetPreviousFailuresByFilterID(filterID)
	hashExclusion := make(map[string]struct{}, len(cs.Provider.TestSuiteKnownFailures))

	for _, v := range cs.Provider.TestSuiteKnownFailures {
		hashExclusion[v] = struct{}{}
	}

	for _, v := range e2eFailuresPipeline {
		ps.Tests[v].State = "filter5KnownFailures"
		if _, ok := hashExclusion[v]; !ok {
			filterFailures = append(filterFailures, v)
			continue
		}
		filterFailuresExcluded = append(filterFailuresExcluded, v)
	}
	sort.Strings(filterFailures)
	ps.SetFailuresByFilterID(filterID, filterFailures, filterFailuresExcluded)

	log.Debugf("Filter (KF) results: plugin=%s in=filter(%d) out=filter(%d) filterExcluded(%d)",
		pluginName, len(e2eFailuresPipeline), len(filterFailures), len(filterFailuresExcluded))
	return nil
}

// Filter6: Replay
// applyFilterReplay skip failures that pass in replay, which can be a
// candidate for flake or false-positive failure.
// Replay step re-runs the failured tests from conformance suites in serial mode,
// to check if the test is passing in a second shot.
func (cs *ConsolidatedSummary) applyFilterReplay(filterID string) error {
	for _, pluginName := range []string{
		plugin.PluginNameKubernetesConformance,
		plugin.PluginNameOpenShiftConformance,
	} {
		if err := cs.applyFilterReplayForPlugin(pluginName, filterID); err != nil {
			return fmt.Errorf("error while processing filter5 (Replay): %w", err)
		}
	}
	return nil
}

// Filter6 by plugin
// applyFilterReplayForPlugin extracts passed tests from replay step, and check
// if conformance plugins has intersection in its failures, if so the test is passing
// in the second run, excluding it from the failures.
func (cs *ConsolidatedSummary) applyFilterReplayForPlugin(pluginName string, filterID string) error {
	var ps *plugin.OPCTPluginSummary
	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultK8SValidated()

	case plugin.PluginNameOpenShiftConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultOCPValidated()

	case plugin.PluginNameOpenShiftUpgrade:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceUpgrade()

	default:
		return fmt.Errorf("plugin not found: %s", pluginName)
	}

	// read the failures from pipeline
	filterFailures, filterFailuresExcluded := ps.GetFailuresByFilterID(filterID)
	e2eFailuresPipeline := ps.GetPreviousFailuresByFilterID(filterID)

	replayPlugin := cs.GetProvider().GetOpenShift().GetResultConformanceReplay()
	if replayPlugin == nil {
		ps.SetFailuresByFilterID(filterID, filterFailures, filterFailuresExcluded)
		log.Debugf("Filter (Replay) results: plugin=%s in=filter(%d) out=filter(%d) filterExcluded(%d)",
			pluginName, len(e2eFailuresPipeline),
			len(filterFailures), len(filterFailuresExcluded))
		log.Debugf("skipping filter (Replay) for plugin: %s, no replay results", pluginName)
		return nil
	}

	passedReplay := make(map[string]struct{}, len(replayPlugin.Tests))
	failedReplay := make(map[string]struct{}, len(replayPlugin.Tests))
	for _, test := range replayPlugin.Tests {
		name := test.Name
		if test.Status == "passed" {
			passedReplay[name] = struct{}{}
			continue
		}
		failedReplay[name] = struct{}{}
	}

	for _, v := range e2eFailuresPipeline {
		ps.Tests[v].State = "filter6Replay"
		if _, ok := passedReplay[v]; !ok {
			filterFailures = append(filterFailures, v)
			continue
		}
		filterFailuresExcluded = append(filterFailuresExcluded, v)
	}
	sort.Strings(filterFailures)
	ps.SetFailuresByFilterID(filterID, filterFailures, filterFailuresExcluded)

	log.Debugf("Filter (Replay) results: plugin=%s in=filter(%d) replay=pass(%d) fail(%d) out=filter(%d) filterExcluded(%d)",
		pluginName, len(e2eFailuresPipeline), len(passedReplay), len(failedReplay),
		len(filterFailures), len(filterFailuresExcluded))
	return nil
}

// Filter Final:
// applyFilterCopyPipeline builds the final failures after filters for each plugin.
func (cs *ConsolidatedSummary) applyFilterCopyPipeline(filterID string) error {
	for _, pluginName := range []string{
		plugin.PluginNameOpenShiftUpgrade,
		plugin.PluginNameKubernetesConformance,
		plugin.PluginNameOpenShiftConformance,
		plugin.PluginNameConformanceReplay,
	} {
		if err := cs.applyFilterCopyPipelineForPlugin(pluginName, filterID); err != nil {
			return fmt.Errorf("error while building filtered failures: %w", err)
		}
	}
	return nil
}

// applyFilterCopyPipelineForPlugin copy the last filter in the pipeline to the final result of failures.
func (cs *ConsolidatedSummary) applyFilterCopyPipelineForPlugin(pluginName string, filterID string) error {
	var ps *plugin.OPCTPluginSummary

	// Get the list of the last filter in the pipeline
	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultK8SValidated()
		// Should point to the last filter in the pipeline.
		ps.FailedFiltered = ps.GetPreviousFailuresByFilterID(filterID)

	case plugin.PluginNameOpenShiftConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultOCPValidated()
		// Should point to the last filter in the pipeline.
		ps.FailedFiltered = ps.GetPreviousFailuresByFilterID(filterID)

	case plugin.PluginNameOpenShiftUpgrade:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceUpgrade()
		// Should point to the last filter in the pipeline.
		ps.FailedFiltered = ps.GetPreviousFailuresByFilterID(filterID)

	case plugin.PluginNameConformanceReplay:
		ps = cs.GetProvider().GetOpenShift().GetResultConformanceReplay()
		// Should point to the last filter in the pipeline.
		ps.FailedFiltered = ps.FailedList

	default:
		return fmt.Errorf("invalid plugin: %s", pluginName)
	}

	log.Debugf("Filter results (Final): plugin=%s filtered failures(%d)", pluginName, len(ps.FailedFiltered))
	return nil
}

// saveResultsPlugin saves the results of the plugin to the disk to be used
// on the review process.
func (cs *ConsolidatedSummary) saveResultsPlugin(path, pluginName string) error {
	var resultsProvider *plugin.OPCTPluginSummary
	var resultsBaseline *plugin.OPCTPluginSummary
	var suite *OpenshiftTestsSuite
	var prefix = "tests"
	bProcessed := cs.GetBaseline().HasValidResults()

	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		resultsProvider = cs.GetProvider().GetOpenShift().GetResultK8SValidated()
		if bProcessed {
			resultsBaseline = cs.GetBaseline().GetOpenShift().GetResultK8SValidated()
		}
		suite = cs.GetProvider().GetSuites().KubernetesConformance
	case plugin.PluginNameOpenShiftConformance:
		resultsProvider = cs.GetProvider().GetOpenShift().GetResultOCPValidated()
		if bProcessed {
			resultsBaseline = cs.GetBaseline().GetOpenShift().GetResultOCPValidated()
		}
		suite = cs.GetProvider().GetSuites().OpenshiftConformance
	}

	if cs.Verbose {
		// Save Provider failures
		filename := fmt.Sprintf("%s/%s_%s_provider_failures-1-ini.txt", path, prefix, pluginName)
		if err := writeFileTestList(filename, resultsProvider.FailedList); err != nil {
			return err
		}

		// Save Provider failures with filter: Suite (only)
		filename = fmt.Sprintf("%s/%s_%s_provider_failures-2-filter1_suite.txt", path, prefix, pluginName)
		if err := writeFileTestList(filename, resultsProvider.FailedFilter1); err != nil {
			return err
		}

		// Save Provider failures with filter: Baseline exclusion
		filename = fmt.Sprintf("%s/%s_%s_provider_failures-3-filter2_baseline.txt", path, prefix, pluginName)
		if err := writeFileTestList(filename, resultsProvider.FailedFilter2); err != nil {
			return err
		}

		// Save Provider failures with filter: Flaky
		filename = fmt.Sprintf("%s/%s_%s_provider_failures-4-filter3_without_flakes.txt", path, prefix, pluginName)
		if err := writeFileTestList(filename, resultsProvider.FailedFilter3); err != nil {
			return err
		}

		// Save Provider failures with filter: Baseline API
		filename = fmt.Sprintf("%s/%s_%s_provider_failures-5-filter4_api.txt", path, prefix, pluginName)
		if err := writeFileTestList(filename, resultsProvider.FailedFilter4); err != nil {
			return err
		}

		// Save Provider failures with filter: Known Failures
		filename = fmt.Sprintf("%s/%s_%s_provider_failures-5-filter5_knownfailures.txt", path, prefix, pluginName)
		if err := writeFileTestList(filename, resultsProvider.FailedFilter5); err != nil {
			return err
		}

		// Save the Providers failures for the latest filter to review (focus on this)
		filename = fmt.Sprintf("%s/%s_%s_provider_failures.txt", path, prefix, pluginName)
		if err := writeFileTestList(filename, resultsProvider.FailedFilter3); err != nil {
			return err
		}

		// Save baseline failures
		if bProcessed {
			filename = fmt.Sprintf("%s/%s_%s_baseline_failures.txt", path, prefix, pluginName)
			if err := writeFileTestList(filename, resultsBaseline.FailedList); err != nil {
				return err
			}
		}

		// Save the openshift-tests suite use by this plugin:
		filename = fmt.Sprintf("%s/%s_%s_suite_full.txt", path, prefix, pluginName)
		if err := writeFileTestList(filename, suite.Tests); err != nil {
			return err
		}
	}
	return nil
}

func (cs *ConsolidatedSummary) extractFailuresDetailsByPlugin(path, pluginName string) error {
	var resultsProvider *plugin.OPCTPluginSummary
	ignoreExistingDir := true

	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		resultsProvider = cs.GetProvider().GetOpenShift().GetResultK8SValidated()
	case plugin.PluginNameOpenShiftConformance:
		resultsProvider = cs.GetProvider().GetOpenShift().GetResultOCPValidated()
	}

	// extract all failed by plugins
	currentDirectory := fmt.Sprintf("failures-%s", pluginName)
	subdir := fmt.Sprintf("%s/%s/", path, currentDirectory)
	if err := createDir(subdir, ignoreExistingDir); err != nil {
		return err
	}
	errFailures := make([]string, len(resultsProvider.Tests))
	for k := range resultsProvider.Tests {
		errFailures = append(errFailures, k)
	}
	if err := extractSaveTestErrors(subdir, resultsProvider.Tests, errFailures); err != nil {
		return err
	}

	return nil
}

// SaveResults dump all the results and processed to the disk to be used
// on the review process.
func (cs *ConsolidatedSummary) SaveResults(path string) error {

	cs.Timers.Add("cs-save/results")
	if err := createDir(path, true); err != nil {
		return err
	}

	// Save the list of failures into individual files by Plugin
	if err := cs.saveResultsPlugin(path, plugin.PluginNameKubernetesConformance); err != nil {
		return err
	}
	if err := cs.saveResultsPlugin(path, plugin.PluginNameOpenShiftConformance); err != nil {
		return err
	}

	// Extract errors details to sub directories
	if err := cs.extractFailuresDetailsByPlugin(path, plugin.PluginNameKubernetesConformance); err != nil {
		return err
	}
	if err := cs.extractFailuresDetailsByPlugin(path, plugin.PluginNameOpenShiftConformance); err != nil {
		return err
	}

	log.Infof("#> Data Saved to directory %q", path)
	cs.Timers.Add("cs-save/results")
	return nil
}

// writeFileTestList saves the list of test names to a new text file
func writeFileTestList(filename string, data []string) error {
	fd, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed creating file: %s", err)
	}
	defer fd.Close()

	writer := bufio.NewWriter(fd)
	defer writer.Flush()

	for _, line := range data {
		_, err = writer.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

// extractTestErrors dumps the test error, summary and stdout, then saved
// to individual files.
func extractSaveTestErrors(prefix string, items plugin.Tests, failures []string) error {

	for _, line := range failures {
		if _, ok := items[line]; ok {
			file := fmt.Sprintf("%s%s-failure.txt", prefix, items[line].ID)
			err := writeErrorToFile(file, items[line].Failure)
			if err != nil {
				log.Errorf("Error writing Failure for test: %s\n", line)
			}

			file = fmt.Sprintf("%s%s-systemOut.txt", prefix, items[line].ID)
			err = writeErrorToFile(file, items[line].SystemOut)
			if err != nil {
				log.Errorf("Error writing SystemOut for test: %s\n", line)
			}
		}
	}
	return nil
}

// writeErrorToFile save the entire buffer to individual file.
func writeErrorToFile(file, data string) error {
	fd, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed creating file: %s", err)
	}
	defer fd.Close()

	writer := bufio.NewWriter(fd)
	defer writer.Flush()

	_, err = writer.WriteString(data)
	if err != nil {
		return err
	}

	return nil
}

// createDir checks if the directory exists, if not creates it, otherwise log and return error
func createDir(path string, ignoreexisting bool) error {
	// Saved directory must be created by must-gather extractor.
	// TODO check cases not covered by that flow.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		if ignoreexisting {
			return nil
		}
		return errors.New(fmt.Sprintf("directory already exists: %s", path))
	}

	if err := os.Mkdir(path, os.ModePerm); err != nil {
		log.Errorf("ERROR: Unable to create directory [%s]: %v", path, err)
		return err
	}
	return nil
}

// applyFilterFlaky process the FailedFilterSuite for each plugin, **excluding** failures from
// baseline test.
func (cs *ConsolidatedSummary) buildDocumentation() error {
	err := cs.buildDocumentationForPlugin(plugin.PluginNameKubernetesConformance)
	if err != nil {
		return err
	}

	err = cs.buildDocumentationForPlugin(plugin.PluginNameOpenShiftConformance)
	if err != nil {
		return err
	}

	return nil
}

// buildDocumentationForPlugin builds the documentation for the test failure for each plugin.
func (cs *ConsolidatedSummary) buildDocumentationForPlugin(pluginName string) error {
	var (
		ps               *plugin.OPCTPluginSummary
		version          string
		docUserBaseURL   string
		docSourceBaseURL string
	)

	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultK8SValidated()
		versionFull := cs.GetProvider().GetSonobuoyCluster().APIVersion
		reVersion := regexp.MustCompile(`^v(\d+\.\d+)`)
		matches := reVersion.FindStringSubmatch(versionFull)
		if len(matches) != 2 {
			log.Warnf("Unable to extract kubernetes version to build documentation: %v [%v]", versionFull, matches)
			return nil
		}
		version = matches[1]
		docUserBaseURL = fmt.Sprintf("https://github.com/cncf/k8s-conformance/blob/master/docs/KubeConformance-%s.md", version)
		docSourceBaseURL = fmt.Sprintf("https://raw.githubusercontent.com/cncf/k8s-conformance/master/docs/KubeConformance-%s.md", version)
	case plugin.PluginNameOpenShiftConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultOCPValidated()
		// OCP tests does not have documentation (TODO: check what can be used)
		// https://docs.openshift.com/container-platform/4.13/welcome/index.html
		// https://access.redhat.com/search/
		docUserBaseURL = "https://github.com/openshift/origin/blob/master/test/extended/README.md"
		docSourceBaseURL = docUserBaseURL
	default:
		return errors.New("Plugin not found to apply filter: Flaky")
	}

	if ps.Documentation == nil {
		ps.Documentation = plugin.NewTestDocumentation(docUserBaseURL, docSourceBaseURL)
		err := ps.Documentation.Load()
		if err != nil {
			return err
		}
		err = ps.Documentation.BuildIndex()
		if err != nil {
			return err
		}
	}

	for _, test := range ps.Tests {
		test.LookupDocumentation(ps.Documentation)
	}

	return nil
}
