package summary

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"

	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/metrics"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/plugin"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/openshift/ci/sippy"
)

// ConsolidatedSummary Aggregate the results of provider and baseline
type ConsolidatedSummary struct {
	Verbose  bool
	Timers   metrics.Timers
	Provider *ResultSummary
	Baseline *ResultSummary
}

// Process entrypoint to read and fill all summaries for each archive, plugin and suites
// applying any transformation it needs through filters.
func (cs *ConsolidatedSummary) Process() error {
	cs.Timers.Add("cs-process")

	// Load Result Summary from Archives
	log.Debug("Processing results/Populating Provider")
	cs.Timers.Set("cs-process/populate-provider")
	if err := cs.Provider.Populate(); err != nil {
		fmt.Println("ERROR processing provider results...")
		return err
	}

	log.Debug("Processing results/Populating Baseline")
	cs.Timers.Set("cs-process/populate-baseline")
	if err := cs.Baseline.Populate(); err != nil {
		fmt.Println("ERROR processing baseline results...")
		return err
	}

	// Filters
	log.Debug("Processing results/Applying filters/Suite")
	cs.Timers.Set("cs-process/filter-suite")
	if err := cs.applyFilterSuite(); err != nil {
		return err
	}

	log.Debug("Processing results/Applying filters/Baseline")
	cs.Timers.Set("cs-process/filter-baseline")
	if err := cs.applyFilterBaseline(); err != nil {
		return err
	}

	log.Debug("Processing results/Applying filters/Flake")
	cs.Timers.Set("cs-process/filter-flake")
	if err := cs.applyFilterFlaky(); err != nil {
		return err
	}

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

// applyFilterSuite process the FailedList for each plugin, getting **intersection** tests
// for respective suite.
func (cs *ConsolidatedSummary) applyFilterSuite() error {
	err := cs.applyFilterSuiteForPlugin(plugin.PluginNameKubernetesConformance)
	if err != nil {
		return err
	}

	err = cs.applyFilterSuiteForPlugin(plugin.PluginNameOpenShiftConformance)
	if err != nil {
		return err
	}

	return nil
}

// applyFilterSuiteForPlugin calculates the intersection of Provider Failed AND suite
func (cs *ConsolidatedSummary) applyFilterSuiteForPlugin(pluginName string) error {

	var resultsProvider *plugin.OPCTPluginSummary
	var pluginSuite *OpenshiftTestsSuite

	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		resultsProvider = cs.GetProvider().GetOpenShift().GetResultK8SValidated()
		pluginSuite = cs.GetProvider().GetSuites().KubernetesConformance
	case plugin.PluginNameOpenShiftConformance:
		resultsProvider = cs.GetProvider().GetOpenShift().GetResultOCPValidated()
		pluginSuite = cs.GetProvider().GetSuites().OpenshiftConformance
	}

	e2eFailures := resultsProvider.FailedList
	e2eSuite := pluginSuite.Tests
	emptySuite := len(pluginSuite.Tests) == 0
	hashSuite := make(map[string]struct{}, len(e2eSuite))

	for _, v := range e2eSuite {
		hashSuite[v] = struct{}{}
	}

	for _, v := range e2eFailures {
		// move on the pipeline when the suite is empty.
		resultsProvider.Tests[v].State = "filterSuiteOnly"
		if emptySuite {
			resultsProvider.FailedFilterSuite = append(resultsProvider.FailedFilterSuite, v)
		} else {
			if _, ok := hashSuite[v]; ok {
				resultsProvider.FailedFilterSuite = append(resultsProvider.FailedFilterSuite, v)
			}
		}
	}
	sort.Strings(resultsProvider.FailedFilterSuite)
	return nil
}

// applyFilterBaseline process the FailedFilterSuite for each plugin, **excluding** failures from
// baseline test.
func (cs *ConsolidatedSummary) applyFilterBaseline() error {
	err := cs.applyFilterBaselineForPlugin(plugin.PluginNameKubernetesConformance)
	if err != nil {
		return err
	}

	err = cs.applyFilterBaselineForPlugin(plugin.PluginNameOpenShiftConformance)
	if err != nil {
		return err
	}

	return nil
}

// applyFilterBaselineForPlugin calculates the **exclusion** tests of
// Provider Failed included on suite and Baseline failed tests.
func (cs *ConsolidatedSummary) applyFilterBaselineForPlugin(pluginName string) error {

	var providerSummary *plugin.OPCTPluginSummary
	var e2eFailuresBaseline []string

	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		providerSummary = cs.GetProvider().GetOpenShift().GetResultK8SValidated()
		if cs.GetBaseline().HasValidResults() {
			e2eFailuresBaseline = cs.GetBaseline().GetOpenShift().GetResultK8SValidated().FailedList
		}
	case plugin.PluginNameOpenShiftConformance:
		providerSummary = cs.GetProvider().GetOpenShift().GetResultOCPValidated()
		if cs.GetBaseline().HasValidResults() {
			e2eFailuresBaseline = cs.GetBaseline().GetOpenShift().GetResultOCPValidated().FailedList
		}
	default:
		return errors.New("Suite not found to apply filter: Flaky")
	}

	e2eFailuresProvider := providerSummary.FailedFilterSuite
	hashBaseline := make(map[string]struct{}, len(e2eFailuresBaseline))

	for _, v := range e2eFailuresBaseline {
		hashBaseline[v] = struct{}{}
	}

	for _, v := range e2eFailuresProvider {
		providerSummary.Tests[v].State = "filterBaseline"
		if _, ok := hashBaseline[v]; !ok {
			providerSummary.FailedFilterBaseline = append(providerSummary.FailedFilterBaseline, v)
		}
	}
	sort.Strings(providerSummary.FailedFilterBaseline)
	return nil
}

// applyFilterFlaky process the FailedFilterSuite for each plugin, **excluding** failures from
// baseline test.
func (cs *ConsolidatedSummary) applyFilterFlaky() error {
	err := cs.applyFilterFlakeForPlugin(plugin.PluginNameKubernetesConformance)
	if err != nil {
		return err
	}

	err = cs.applyFilterFlakeForPlugin(plugin.PluginNameOpenShiftConformance)
	if err != nil {
		return err
	}

	return nil
}

// applyFilterFlakeForPlugin query the Sippy API looking for each failed test
// on each plugin/suite, saving the list on the ResultSummary.
func (cs *ConsolidatedSummary) applyFilterFlakeForPlugin(pluginName string) error {

	var ps *plugin.OPCTPluginSummary

	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultK8SValidated()
	case plugin.PluginNameOpenShiftConformance:
		ps = cs.GetProvider().GetOpenShift().GetResultOCPValidated()
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
	for _, name := range ps.FailedFilterBaseline {
		ps.Tests[name].State = "filterFlake"
		resp, err := api.QueryTests(&sippy.SippyTestsRequestInput{TestName: name})
		if err != nil {
			log.Errorf("#> Error querying to Sippy API: %v", err)
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
			if ps.Tests[name].Flake.CurrentFlakePerc <= 5 {
				ps.Tests[name].State = "filterPriority"
				ps.FailedFilterPrio = append(ps.FailedFilterPrio, name)
			}
		}
	}

	sort.Strings(ps.FailedFilterPrio)
	return nil
}

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
		if err := writeFileTestList(filename, resultsProvider.FailedFilterSuite); err != nil {
			return err
		}

		// Save Provider failures with filter: Baseline exclusion
		filename = fmt.Sprintf("%s/%s_%s_provider_failures-3-filter2_baseline.txt", path, prefix, pluginName)
		if err := writeFileTestList(filename, resultsProvider.FailedFilterBaseline); err != nil {
			return err
		}

		// Save Provider failures with filter: Flaky
		filename = fmt.Sprintf("%s/%s_%s_provider_failures-4-filter3_without_flakes.txt", path, prefix, pluginName)
		if err := writeFileTestList(filename, resultsProvider.FailedFilterPrio); err != nil {
			return err
		}

		// Save the Providers failures for the latest filter to review (focus on this)
		filename = fmt.Sprintf("%s/%s_%s_provider_failures.txt", path, prefix, pluginName)
		if err := writeFileTestList(filename, resultsProvider.FailedFilterBaseline); err != nil {
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
	// var resultsBaseline *OPCTPluginSummary
	// bProcessed := cs.GetBaseline().HasValidResults()
	ignoreExistingDir := true

	switch pluginName {
	case plugin.PluginNameKubernetesConformance:
		resultsProvider = cs.GetProvider().GetOpenShift().GetResultK8SValidated()
		// if bProcessed {
		// 	resultsBaseline = cs.GetBaseline().GetOpenShift().GetResultK8SValidated()
		// }
	case plugin.PluginNameOpenShiftConformance:
		resultsProvider = cs.GetProvider().GetOpenShift().GetResultOCPValidated()
		// if bProcessed {
		// 	resultsBaseline = cs.GetBaseline().GetOpenShift().GetResultOCPValidated()
		// }
	}

	// currentDirectory := "failures-provider-filtered"
	// subdir := fmt.Sprintf("%s/%s", path, currentDirectory)
	// if err := createDir(subdir, ignoreExistingDir); err != nil {
	// 	return err
	// }

	// subPrefix := fmt.Sprintf("%s/%s", subdir, plugin)
	// errItems := resultsProvider.FailedItems
	// errList := resultsProvider.FailedFilterBaseline
	// if err := extractSaveTestErrors(subPrefix, errItems, errList); err != nil {
	// 	return err
	// }

	// currentDirectory = "failures-provider"
	// subdir = fmt.Sprintf("%s/%s", path, currentDirectory)
	// if err := createDir(subdir, ignoreExistingDir); err != nil {
	// 	return err
	// }

	// subPrefix = fmt.Sprintf("%s/%s", subdir, plugin)
	// errItems = resultsProvider.FailedItems
	// errList = resultsProvider.FailedList
	// if err := extractSaveTestErrors(subPrefix, errItems, errList); err != nil {
	// 	return err
	// }

	// currentDirectory = "failures-baseline"
	// subdir = fmt.Sprintf("%s/%s", path, currentDirectory)
	// if err := createDir(subdir, ignoreExistingDir); err != nil {
	// 	return err
	// }

	// if bProcessed {
	// 	subPrefix = fmt.Sprintf("%s/%s", subdir, plugin)
	// 	errItems = resultsBaseline.FailedItems
	// 	errList = resultsBaseline.FailedList
	// 	if err := extractSaveTestErrors(subPrefix, errItems, errList); err != nil {
	// 		return err
	// 	}
	// }

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

	fmt.Printf("\n Data Saved to directory '%s'\n", path)
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

// applyFilterFlakeForPlugin query the Sippy API looking for each failed test
// on each plugin/suite, saving the list on the ResultSummary.
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
		// fmt.Println(matches)
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
