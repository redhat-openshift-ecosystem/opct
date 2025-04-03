package summary

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/archive"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/redhat-openshift-ecosystem/opct/internal/openshift/mustgather"
	"github.com/redhat-openshift-ecosystem/opct/internal/openshift/mustgathermetrics"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/vmware-tanzu/sonobuoy/pkg/client/results"
	"github.com/vmware-tanzu/sonobuoy/pkg/discovery"
)

const (
	ResultSourceNameProvider = "provider"
	ResultSourceNameBaseline = "baseline"
)

// ResultSummary persists the reference of results archive.
type ResultSummary struct {
	Name      string
	Archive   string
	Sonobuoy  *SonobuoySummary
	OpenShift *OpenShiftSummary
	Suites    *OpenshiftTestsSuites

	// isConformance indicates if it is a conformance plugin when true.
	isConformance bool

	// reader is a file description for the archive tarball.
	reader *results.Reader

	// SavePath is the target path to save the extracted report.
	SavePath string

	// MustGather stores the extracted items from must-gather.
	MustGather *mustgather.MustGather

	HasCAMGI         bool
	HasMetrics       bool
	HasInstallConfig bool

	// Metrics stores the extracted items from must-gather metrics.
	Metrics *mustgathermetrics.MustGatherMetrics

	// Plugin Know failures
	TestSuiteKnownFailures []string

	// BaselineAPI holds the data fetched from the baseline API.
	BaselineAPI string
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
		// log.Warnf("Ignoring to populate source '%s'. Missing or invalid baseline artifact (-b): %s", rs.Name, rs.Archive)
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
	for _, pluginName := range plugins {
		log.Infof("Processing Plugin %s...", pluginName)
		switch pluginName {
		case plugin.PluginNameKubernetesConformance, plugin.PluginNameOpenShiftConformance:
			rs.isConformance = true
		}

		log.Debugf("Processing results/Populating/Processing Plugin/%s", pluginName)
		if err := rs.processPlugin(pluginName); err != nil {
			log.Errorf("Processing results/Populating/Processing Plugin/%s: %v", pluginName, err)
			lastErr = err
		}
	}

	log.Info("Processing results...")
	cleanup, err = rs.openReader()
	defer cleanup()
	if err != nil {
		return err
	}

	log.Debugf("Processing results/Populating/Populating Summary")
	err = rs.extractAndLoadData()
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
func (rs *ResultSummary) processPlugin(pluginName string) error {

	// TODO: review the fd usage for tarbal and file
	cleanup, err := rs.openReader()
	defer cleanup()
	if err != nil {
		return err
	}

	obj, err := rs.reader.PluginResultsItem(pluginName)
	if err != nil {
		return err
	}

	if err = rs.processPluginResult(obj); err != nil {
		return err
	}
	return nil
}

// processPluginResult receives the plugin results object and parse it to the summary.
func (rs *ResultSummary) processPluginResult(obj *results.Item) error {
	statusCounts := map[string]int{}
	var tests []results.Item
	var failures []string

	statusCounts, tests = walkForSummary(obj, statusCounts, tests)

	total := 0
	for _, v := range statusCounts {
		total += v
	}

	testItems := make(map[string]*plugin.TestItem, len(tests))
	for idx, item := range tests {
		testItems[item.Name] = &plugin.TestItem{
			Name:  item.Name,
			ID:    fmt.Sprintf("%s-%d", obj.Name, idx),
			State: "processed",
		}
		if item.Status != "" {
			testItems[item.Name].Status = item.Status
		}
		switch item.Status {
		case results.StatusFailed, results.StatusTimeout:
			if _, ok := item.Details["failure"]; ok {
				testItems[item.Name].Failure = item.Details["failure"].(string)
			}
			if _, ok := item.Details["system-out"]; ok {
				testItems[item.Name].SystemOut = item.Details["system-out"].(string)
			}
			if _, ok := item.Details["offset"]; ok {
				testItems[item.Name].Offset = item.Details["offset"].(int)
			}
			failures = append(failures, item.Name)
			testItems[item.Name].UpdateErrorCounter()
		}
	}

	if err := rs.GetOpenShift().SetPluginResult(&plugin.OPCTPluginSummary{
		Name:       obj.Name,
		Status:     obj.Status,
		Total:      int64(total),
		Passed:     int64(statusCounts[results.StatusPassed]),
		Failed:     int64(statusCounts[results.StatusFailed] + statusCounts[results.StatusTimeout]),
		Timeout:    int64(statusCounts[results.StatusTimeout]),
		Skipped:    int64(statusCounts[results.StatusSkipped]),
		FailedList: failures,
		Tests:      testItems,
	}); err != nil {
		return err
	}

	delete(statusCounts, results.StatusPassed)
	delete(statusCounts, results.StatusFailed)
	delete(statusCounts, results.StatusTimeout)
	delete(statusCounts, results.StatusSkipped)

	return nil
}

// extractDataFromTarball load all files from archive reader and extract desired
// information to the ResultSummary.
func (rs *ResultSummary) extractAndLoadData() error {
	// Path to files insides Sonobuoy tarball
	const (
		// OpenShift objects files in archive collected by aggregator server
		pathResourceInfrastructures  = "resources/cluster/config.openshift.io_v1_infrastructures.json"
		pathResourceClusterVersions  = "resources/cluster/config.openshift.io_v1_clusterversions.json"
		pathResourceClusterOperators = "resources/cluster/config.openshift.io_v1_clusteroperators.json"
		pathResourceClusterNetwork   = "resources/cluster/config.openshift.io_v1_networks.json"

		// Kuberenetes resources locations on archive file
		pathResourceNodes = "resources/cluster/core_v1_nodes.json"

		// Sonobuoy files in archive
		// Sonobuoy metadata files
		pathMetaRun    = "meta/run.log"
		pathMetaConfig = "meta/config.json"

		// Sonobuoy plugin files
		pathPluginDefinition10 = "plugins/10-openshift-kube-conformance/definition.json"
		pathPluginDefinition20 = "plugins/20-openshift-conformance-validated/definition.json"

		pathResourceNSOpctConfigMap = "resources/ns/opct/core_v1_configmaps.json"
		pathResourceNsKubeConfigMap = "resources/ns/kube-system/core_v1_configmaps.json"
		pathResourceNsOcpConfigMap  = "resources/ns/openshift-config/core_v1_configmaps.json"

		// artifacts collector locations on archive file
		pathPluginArtifactTestsK8S     = "plugins/99-openshift-artifacts-collector/results/global/artifacts_e2e-tests_openshift-kube-conformance.txt"
		pathPluginArtifactTestsOCP     = "plugins/99-openshift-artifacts-collector/results/global/artifacts_e2e-tests_openshift-conformance-validated.txt"
		pathPluginArtifactTestsUpgrade = "plugins/99-openshift-artifacts-collector/results/global/artifacts_e2e-tests_openshift-cluster-upgrade.txt"
		pathPluginArtifactTestsReplay  = "plugins/99-openshift-artifacts-collector/results/global/artifacts_e2e-tests_openshift-tests-replay.txt"
		pathCAMIG                      = "plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather_camgi.html"
		pathMetrics                    = "plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather-metrics.tar.xz"

		// TODO: the following file is used to keep compatibility with versions older than v0.3
		pathPluginArtifactTestsOCP2 = "plugins/99-openshift-artifacts-collector/results/global/artifacts_e2e-openshift-conformance.txt"
		pathMustGather              = "plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather.tar.xz"
	)

	// Data bindings
	mustGather := bytes.Buffer{}
	saveToFlagEnabled := rs.SavePath != ""
	testsSuiteK8S := bytes.Buffer{}
	testsSuiteOCP := bytes.Buffer{}

	CAMGI := bytes.Buffer{}
	MetricsData := bytes.Buffer{}

	metaRunLogs := bytes.Buffer{}
	metaConfig := archive.MetaConfigSonobuoy{}

	sbCluster := discovery.ClusterSummary{}
	ocpInfra := configv1.InfrastructureList{}
	ocpCV := configv1.ClusterVersionList{}
	ocpCO := configv1.ClusterOperatorList{}
	ocpCN := configv1.NetworkList{}
	opctConfigMapList := v1.ConfigMapList{}
	kubeSystemConfigMapList := v1.ConfigMapList{}
	openshiftConfigMapList := v1.ConfigMapList{}
	nodes := v1.NodeList{}

	pluginDef10 := SonobuoyPluginDefinition{}
	pluginDef20 := SonobuoyPluginDefinition{}

	if rs.SavePath != "" {
		log.Debugf("Creating output directory %s...", rs.SavePath)
		if err := os.MkdirAll(rs.SavePath, os.ModePerm); err != nil {
			log.Errorf("Unable to create directory %s: %v", rs.SavePath, err)
		}
	}

	patternPluginLogs := `^podlogs\/.*\/sonobuoy-.*-job-.*\/logs\/plugin.txt`
	rePluginLogs := regexp.MustCompile(patternPluginLogs)

	// Iterate over the archive to get the items as an object to build the Summary report.
	log.Debugf("Processing results/Populating/Populating Summary/Extracting")
	err := rs.reader.WalkFiles(func(path string, info os.FileInfo, e error) error {
		// Extract and marshal the files into the structures
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
		if err := results.ExtractFileIntoStruct(pathResourceClusterNetwork, path, info, &ocpCN); err != nil {
			return errors.Wrap(err, fmt.Sprintf("extracting file '%s': %v", path, err))
		}
		if err := results.ExtractFileIntoStruct(pathPluginDefinition10, path, info, &pluginDef10); err != nil {
			return errors.Wrap(err, fmt.Sprintf("extracting file '%s': %v", path, err))
		}
		if err := results.ExtractFileIntoStruct(pathPluginDefinition20, path, info, &pluginDef20); err != nil {
			return errors.Wrap(err, fmt.Sprintf("extracting file '%s': %v", path, err))
		}
		if err := results.ExtractFileIntoStruct(pathMetaConfig, path, info, &metaConfig); err != nil {
			return errors.Wrap(err, fmt.Sprintf("extracting file '%s': %v", path, err))
		}
		if err := results.ExtractFileIntoStruct(pathResourceNSOpctConfigMap, path, info, &opctConfigMapList); err != nil {
			return errors.Wrap(err, fmt.Sprintf("extracting file '%s': %v", path, err))
		}
		if err := results.ExtractFileIntoStruct(pathResourceNodes, path, info, &nodes); err != nil {
			return errors.Wrap(err, fmt.Sprintf("extracting file '%s': %v", path, err))
		}
		if err := results.ExtractFileIntoStruct(pathResourceNsKubeConfigMap, path, info, &kubeSystemConfigMapList); err != nil {
			return errors.Wrap(err, fmt.Sprintf("extracting file '%s': %v", path, err))
		}
		if err := results.ExtractFileIntoStruct(pathResourceNsOcpConfigMap, path, info, &openshiftConfigMapList); err != nil {
			return errors.Wrap(err, fmt.Sprintf("extracting file '%s': %v", path, err))
		}
		// Extract raw files
		if warn := results.ExtractBytes(pathPluginArtifactTestsK8S, path, info, &testsSuiteK8S); warn != nil {
			log.Warnf("Unable to load file %s: %v\n", pathPluginArtifactTestsK8S, warn)
			return errors.Wrap(warn, fmt.Sprintf("extracting file '%s': %v", path, warn))
		}
		if warn := results.ExtractBytes(pathPluginArtifactTestsOCP, path, info, &testsSuiteOCP); warn != nil {
			log.Warnf("Unable to load file %s: %v\n", pathPluginArtifactTestsOCP, warn)
			return errors.Wrap(warn, fmt.Sprintf("extracting file '%s': %v", path, warn))
		}
		if warn := results.ExtractBytes(pathPluginArtifactTestsOCP2, path, info, &testsSuiteOCP); warn != nil {
			log.Warnf("Unable to load file %s: %v\n", pathPluginArtifactTestsOCP2, warn)
			return errors.Wrap(warn, fmt.Sprintf("extracting file '%s': %v", path, warn))
		}
		if warn := results.ExtractBytes(pathMetaRun, path, info, &metaRunLogs); warn != nil {
			log.Warnf("Unable to load file %s: %v\n", pathMetaRun, warn)
			return errors.Wrap(warn, fmt.Sprintf("extracting file '%s': %v", path, warn))
		}
		if warn := results.ExtractBytes(pathMustGather, path, info, &mustGather); warn != nil {
			log.Warnf("Unable to load file %s: %v\n", pathMustGather, warn)
			return errors.Wrap(warn, fmt.Sprintf("extracting file '%s': %v", path, warn))
		}
		if saveToFlagEnabled {
			if warn := results.ExtractBytes(pathCAMIG, path, info, &CAMGI); warn != nil {
				log.Warnf("Unable to load file %s: %v\n", pathCAMIG, warn)
				return errors.Wrap(warn, fmt.Sprintf("extracting file '%s': %v", path, warn))
			}
			if warn := results.ExtractBytes(pathMetrics, path, info, &MetricsData); warn != nil {
				log.Warnf("Unable to load file %s: %v\n", pathCAMIG, warn)
				return errors.Wrap(warn, fmt.Sprintf("extracting file '%s': %v", path, warn))
			}
			// extract podLogs, container plugin
			if rePluginLogs.MatchString(path) {
				var raw bytes.Buffer
				if warn := results.ExtractBytes(path, path, info, &raw); warn != nil {
					log.Warnf("Unable to load plugin log %s: %v\n", path, warn)
					return errors.Wrap(warn, fmt.Sprintf("extracting file '%s': %v", path, warn))
				}
				prefix := strings.Split(path, "-job-")
				if len(prefix) != 2 {
					log.Warnf("Unable to read podLog prefix for path: %s\n", path)
					return nil
				}
				filepath := strings.Split(prefix[0], "/")
				if len(filepath) <= 0 {
					log.Warnf("Unable to read podLog file for path: %s\n", path)
					return nil
				}
				dest := fmt.Sprintf("%s/log-%s-plugin.txt", rs.SavePath, filepath[len(filepath)-1])
				err := os.WriteFile(dest, raw.Bytes(), 0644)
				if err != nil {
					log.Errorf("Processing results/Populating/Populating Summary/Extracting/podLogs/plugins: %v", err)
					return nil
				}
			}
		}
		return e
	})
	if err != nil {
		log.Warnf("Processing results/Populating/Populating Summary/Extracting/result: %v", err)
	}

	log.Debugf("Processing results/Populating/Populating Summary/Processing")
	if err := rs.GetSonobuoy().SetCluster(&sbCluster); err != nil {
		log.Warnf("Processing results/Populating/Populating Summary/Processing/Sonobuoy: %v", err)
	}
	if err := rs.GetOpenShift().SetInfrastructure(&ocpInfra); err != nil {
		log.Warnf("Processing results/Populating/Populating Summary/Processing/Object/Infrastructure: %v", err)
	}
	if err := rs.GetOpenShift().SetClusterVersion(&ocpCV); err != nil {
		log.Warnf("Processing results/Populating/Populating Summary/Processing/Object/Version: %v", err)
	}
	if err := rs.GetOpenShift().SetClusterOperators(&ocpCO); err != nil {
		log.Warnf("Processing results/Populating/Populating Summary/Processing/Object/Operators: %v", err)
	}
	if err := rs.GetOpenShift().SetClusterNetwork(&ocpCN); err != nil {
		log.Warnf("Processing results/Populating/Populating Summary/Processing/Object/Network: %v", err)
	}
	if err := rs.GetOpenShift().SetNodes(&nodes); err != nil {
		log.Warnf("Processing results/Populating/Populating Summary/Processing/Object/Nodes: %v", err)
	}
	if err := rs.GetOpenShift().ExtractOpenShiftConfigMap(&openshiftConfigMapList); err != nil {
		log.Warnf("Processing results/Populating/Populating Summary/Processing/Object/OpenShiftConfigInstallManifest: %v", err)
	}
	if err := rs.Suites.KubernetesConformance.Load(pathPluginArtifactTestsK8S, &testsSuiteK8S); err != nil {
		log.Warnf("Processing results/Populating/Populating Summary/Processing/Plugin/kube: %v", err)
	}
	if err := rs.Suites.OpenshiftConformance.Load(pathPluginArtifactTestsOCP, &testsSuiteOCP); err != nil {
		log.Warnf("Processing results/Populating/Populating Summary/Processing/Plugin/openshift: %v", err)
	}
	rs.GetSonobuoy().SetPluginDefinition(plugin.PluginNameKubernetesConformance, &pluginDef10)
	rs.GetSonobuoy().SetPluginDefinition(plugin.PluginNameOpenShiftConformance, &pluginDef20)

	rs.GetSonobuoy().ParseMetaRunlogs(&metaRunLogs)
	rs.GetSonobuoy().ParseMetaConfig(&metaConfig)
	rs.GetSonobuoy().ParseOpctConfigMap(&opctConfigMapList)

	// TODO the must-gather parser is consuming more resource than expected, need to be
	// reviewed, and parsers and queue handlers refactored.
	log.Debugf("Processing results/Populating/Populating Summary/Processing/MustGather")
	rs.MustGather = mustgather.NewMustGather(fmt.Sprintf("%s/must-gather", rs.SavePath), saveToFlagEnabled)
	if err := rs.MustGather.Process(&mustGather); err != nil {
		log.Errorf("Processing results/Populating/Populating Summary/Processing/MustGather: %v", err)
	} else {
		log.Debugf("Processing results/Populating/Populating Summary/Processing/MustGather/CalculatingErrors")
		rs.MustGather.AggregateCounters()
	}

	if saveToFlagEnabled {
		if len(CAMGI.Bytes()) > 0 {
			err = os.WriteFile(fmt.Sprintf("%s/%s", rs.SavePath, filepath.Base(pathCAMIG)), CAMGI.Bytes(), 0644)
			if err != nil {
				log.Errorf("Processing results/Populating/Populating Summary/Processing/CAMGI: %v", err)
			} else {
				rs.HasCAMGI = true
			}
		} else {
			log.Error("Processing results/Populating/Populating Summary/Processing/CAMGI: Not Found")
		}
		if len(MetricsData.Bytes()) > 0 {
			rs.Metrics, err = mustgathermetrics.NewMustGatherMetrics(rs.SavePath+"/metrics", pathMetrics, "/metrics", &MetricsData)
			if err != nil {
				log.Errorf("Processing results/Populating/Populating Summary/Processing/MetricsData: %v", err)
			} else {
				err := rs.Metrics.Process()
				if err != nil {
					log.Errorf("Processing MetricsData: %v", err)
				}
				rs.HasMetrics = true
			}
		} else {
			log.Error("Processing results/Populating/Populating Summary/Processing/MetricsData: Not Found")
		}
		// extract install-config
		if len(kubeSystemConfigMapList.Items) > 0 {
			for _, config := range kubeSystemConfigMapList.Items {
				if config.ObjectMeta.Name == "cluster-config-v1" {
					dest := fmt.Sprintf("%s/install-config.txt", rs.SavePath)
					err := os.WriteFile(dest, []byte(config.Data["install-config"]), 0644)
					if err != nil {
						log.Errorf("Processing results/Populating/Populating Summary/Extracting/install-config: %v", err)
					}
					rs.HasInstallConfig = true
				}
			}
		}
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
	}

	failList = append(failList, *result)
	return statusCounts, failList
}
