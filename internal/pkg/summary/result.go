package summary

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/vmware-tanzu/sonobuoy/pkg/client/results"
	"github.com/vmware-tanzu/sonobuoy/pkg/discovery"
)

const (
	ResultSourceNameProvider = "provider"
	ResultSourceNameBaseline = "baseline"

	// OpenShift Custom Resources locations on archive file
	pathResourceInfrastructures  = "resources/cluster/config.openshift.io_v1_infrastructures.json"
	pathResourceClusterVersions  = "resources/cluster/config.openshift.io_v1_clusterversions.json"
	pathResourceClusterOperators = "resources/cluster/config.openshift.io_v1_clusteroperators.json"
	pathPluginArtifactTestsK8S   = "plugins/99-openshift-artifacts-collector/results/global/artifacts_e2e-tests_kubernetes-conformance.txt"
	pathPluginArtifactTestsOCP   = "plugins/99-openshift-artifacts-collector/results/global/artifacts_e2e-tests_openshift-conformance.txt"
	// TODO: the following file is used to keep compatibility with versions older than v0.3
	pathPluginArtifactTestsOCP2 = "plugins/99-openshift-artifacts-collector/results/global/artifacts_e2e-openshift-conformance.txt"
)

// ResultSummary persists the reference of resulta archive
type ResultSummary struct {
	Name      string
	Archive   string
	Sonobuoy  *SonobuoySummary
	OpenShift *OpenShiftSummary
	Suites    *OpenshiftTestsSuites
	reader    *results.Reader
}

// HasValidResults checks if the result instance has valid archive to be processed,
// returning true if it's valid.
// Invalid results happens when the baseline archive was not set on the CLI arguments,
// making the 'process' command to ignore the comparisons and filters related.
func (rs *ResultSummary) HasValidResults() bool {
	if rs.Archive == "" && rs.Name == ResultSourceNameBaseline {
		return false
	}
	return true
}

// Populate open the archive and process the files to populate the summary structures.
func (rs *ResultSummary) Populate() error {

	if !rs.HasValidResults() {
		log.Warnf("Ignoring to populate source '%s'. Missing or invalid baseline artifact (-b): %s", rs.Name, rs.Archive)
		return nil
	}

	cleanup, err := rs.openReader()
	defer cleanup()
	if err != nil {
		return errors.Wrapf(err, "unable to open reader for file '%s'", rs.Archive)
	}

	// Report on all plugins or the specified one.
	plugins, err := rs.getPluginList()
	if err != nil {
		return errors.Wrapf(err, "unable to determine plugins to report on")
	}
	if len(plugins) == 0 {
		return fmt.Errorf("no plugins specified by either the --plugin flag or tarball metadata")
	}

	var lastErr error
	for _, plugin := range plugins {
		log.Infof("Processing Plugin %s...\n", plugin)
		switch plugin {
		case PluginNameOpenShiftUpgrade, PluginNameArtifactsCollector:
			log.Infof("Ignoring Plugin %s", plugin)
			continue
		}
		err := rs.processPlugin(plugin)
		if err != nil {
			lastErr = err
		}
	}

	// TODO: review the fd usage for tarbal and file
	cleanup, err = rs.openReader()
	defer cleanup()
	if err != nil {
		return err
	}

	err = rs.populateSummary()
	if err != nil {
		lastErr = err
	}

	return lastErr
}

// GetOpenShift returns the OpenShift objects parsed from results
func (rs *ResultSummary) GetOpenShift() *OpenShiftSummary {
	if !rs.HasValidResults() {
		return &OpenShiftSummary{}
	}
	return rs.OpenShift
}

// GetSonobuoy returns the Sonobuoy objects parsed from results
func (rs *ResultSummary) GetSonobuoy() *SonobuoySummary {
	if !rs.HasValidResults() {
		return &SonobuoySummary{}
	}
	return rs.Sonobuoy
}

// GetSonobuoyCluster returns the SonobuoyCluster object parsed from results
func (rs *ResultSummary) GetSonobuoyCluster() *discovery.ClusterSummary {
	if !rs.HasValidResults() {
		return &discovery.ClusterSummary{}
	}
	return rs.Sonobuoy.Cluster
}

// GetSuites returns the Conformance suites collected from results
func (rs *ResultSummary) GetSuites() *OpenshiftTestsSuites {
	return rs.Suites
}

// getPluginList extract the plugin list from the archive reader.
func (rs *ResultSummary) getPluginList() ([]string, error) {
	runInfo := discovery.RunInfo{}
	err := rs.reader.WalkFiles(func(path string, info os.FileInfo, err error) error {
		return results.ExtractFileIntoStruct(rs.reader.RunInfoFile(), path, info, &runInfo)
	})

	return runInfo.LoadedPlugins, errors.Wrap(err, "finding plugin list")
}

// openReader returns a *results.Reader along with a cleanup function to close the
// underlying readers. The cleanup function is guaranteed to never be nil.
func (rs *ResultSummary) openReader() (func(), error) {

	filepath := rs.Archive
	fi, err := os.Stat(filepath)
	if err != nil {
		rs.reader = nil
		return func() {}, err
	}
	// When results is a directory
	if fi.IsDir() {
		rs.reader = results.NewReaderFromDir(filepath)
		return func() {}, nil
	}
	f, err := os.Open(filepath)
	if err != nil {
		rs.reader = nil
		return func() {}, errors.Wrapf(err, "could not open sonobuoy archive: %v", filepath)
	}

	gzr, err := gzip.NewReader(f)
	if err != nil {
		rs.reader = nil
		return func() { f.Close() }, errors.Wrap(err, "could not make a gzip reader")
	}

	rs.reader = results.NewReaderWithVersion(gzr, results.VersionTen)
	return func() { gzr.Close(); f.Close() }, nil
}

// processPlugin receives the plugin name and load the result file to be processed.
func (rs *ResultSummary) processPlugin(plugin string) error {

	// TODO: review the fd usage for tarbal and file
	cleanup, err := rs.openReader()
	defer cleanup()
	if err != nil {
		return err
	}

	obj, err := rs.reader.PluginResultsItem(plugin)
	if err != nil {
		return err
	}

	err = rs.processPluginResult(obj)
	if err != nil {
		return err
	}
	return nil
}

// processPluginResult receives the plugin results object and parse it to the summary.
func (rs *ResultSummary) processPluginResult(obj *results.Item) error {
	statusCounts := map[string]int{}
	var failures []results.Item
	var failedList []string

	statusCounts, failures = walkForSummary(obj, statusCounts, failures)

	total := 0
	for _, v := range statusCounts {
		total += v
	}

	failedItems := make(map[string]*PluginFailedItem, len(failures))
	for _, item := range failures {
		failedItems[item.Name] = &PluginFailedItem{
			Name: item.Name,
		}
		if _, ok := item.Details["failure"]; ok {
			failedItems[item.Name].Failure = item.Details["failure"].(string)
		}
		if _, ok := item.Details["system-out"]; ok {
			failedItems[item.Name].SystemOut = item.Details["system-out"].(string)
		}
		if _, ok := item.Details["offset"]; ok {
			failedItems[item.Name].Offset = item.Details["offset"].(int)
		}
		failedList = append(failedList, item.Name)
	}

	if err := rs.GetOpenShift().SetPluginResult(&OPCTPluginSummary{
		Name:        obj.Name,
		Status:      obj.Status,
		Total:       int64(total),
		Passed:      int64(statusCounts[results.StatusPassed]),
		Failed:      int64(statusCounts[results.StatusFailed] + statusCounts[results.StatusTimeout]),
		Timeout:     int64(statusCounts[results.StatusTimeout]),
		Skipped:     int64(statusCounts[results.StatusSkipped]),
		FailedList:  failedList,
		FailedItems: failedItems,
	}); err != nil {
		return err
	}

	delete(statusCounts, results.StatusPassed)
	delete(statusCounts, results.StatusFailed)
	delete(statusCounts, results.StatusTimeout)
	delete(statusCounts, results.StatusSkipped)

	return nil
}

// populateSummary load all files from archive reader and extract desired
// information to the ResultSummary.
func (rs *ResultSummary) populateSummary() error {

	var bugSuiteK8S bytes.Buffer
	var bugSuiteOCP bytes.Buffer
	sbCluster := discovery.ClusterSummary{}
	ocpInfra := configv1.InfrastructureList{}
	ocpCV := configv1.ClusterVersionList{}
	ocpCO := configv1.ClusterOperatorList{}

	// Iterate over the archive to get the items as an object to build the Summary report.
	err := rs.reader.WalkFiles(func(path string, info os.FileInfo, e error) error {
		if err := results.ExtractFileIntoStruct(results.ClusterHealthFilePath(), path, info, &sbCluster); err != nil {
			return errors.Wrap(err, fmt.Sprintf("extracting file '%s': %v", path, err))
		}
		if err := results.ExtractFileIntoStruct(pathResourceInfrastructures, path, info, &ocpInfra); err != nil {
			return errors.Wrap(err, fmt.Sprintf("extracting file '%s': %v", path, err))
		}
		if err := results.ExtractFileIntoStruct(pathResourceClusterVersions, path, info, &ocpCV); err != nil {
			return errors.Wrap(err, fmt.Sprintf("extracting file '%s': %v", path, err))
		}
		if err := results.ExtractFileIntoStruct(pathResourceClusterOperators, path, info, &ocpCO); err != nil {
			return errors.Wrap(err, fmt.Sprintf("extracting file '%s': %v", path, err))
		}
		if warn := results.ExtractBytes(pathPluginArtifactTestsK8S, path, info, &bugSuiteK8S); warn != nil {
			log.Warnf("Unable to load file %s: %v\n", pathPluginArtifactTestsK8S, warn)
			return errors.Wrap(warn, fmt.Sprintf("extracting file '%s': %v", path, warn))
		}
		if warn := results.ExtractBytes(pathPluginArtifactTestsOCP, path, info, &bugSuiteOCP); warn != nil {
			log.Warnf("Unable to load file %s: %v\n", pathPluginArtifactTestsOCP, warn)
			return errors.Wrap(warn, fmt.Sprintf("extracting file '%s': %v", path, warn))
		}
		if warn := results.ExtractBytes(pathPluginArtifactTestsOCP2, path, info, &bugSuiteOCP); warn != nil {
			log.Warnf("Unable to load file %s: %v\n", pathPluginArtifactTestsOCP2, warn)
			return errors.Wrap(warn, fmt.Sprintf("extracting file '%s': %v", path, warn))
		}
		return e
	})
	if err != nil {
		return err
	}

	if err := rs.GetSonobuoy().SetCluster(&sbCluster); err != nil {
		return err
	}
	if err := rs.GetOpenShift().SetInfrastructure(&ocpInfra); err != nil {
		return err
	}
	if err := rs.GetOpenShift().SetClusterVersion(&ocpCV); err != nil {
		return err
	}
	if err := rs.GetOpenShift().SetClusterOperators(&ocpCO); err != nil {
		return err
	}
	if err := rs.Suites.KubernetesConformance.Load(pathPluginArtifactTestsK8S, &bugSuiteK8S); err != nil {
		return err
	}
	if err := rs.Suites.OpenshiftConformance.Load(pathPluginArtifactTestsOCP, &bugSuiteOCP); err != nil {
		return err
	}

	return nil
}

// walkForSummary recursively walk through the result YAML file extracting the counters
// and failures.
func walkForSummary(result *results.Item, statusCounts map[string]int, failList []results.Item) (map[string]int, []results.Item) {
	if len(result.Items) > 0 {
		for _, item := range result.Items {
			statusCounts, failList = walkForSummary(&item, statusCounts, failList)
		}
		return statusCounts, failList
	}

	statusCounts[result.Status]++

	if result.Status == results.StatusFailed || result.Status == results.StatusTimeout {
		result.Details["offset"] = statusCounts[result.Status]
		failList = append(failList, *result)
	}

	return statusCounts, failList
}
