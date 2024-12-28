// Package report implements the data layer to extract required information
// to create the report data (json, and viewes).
// It uses the data from the summary package to create the report data.

package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"

	vfs "github.com/redhat-openshift-ecosystem/opct/internal/assets"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/archive"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/metrics"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/summary"
	"github.com/redhat-openshift-ecosystem/opct/internal/openshift/mustgather"
	log "github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/sonobuoy/pkg/discovery"
)

const (
	ReportFileNameIndexJSON = "/opct-report.json"
	// ReportFileNameSummaryJSON is used to API to apply diffs and filters, consumed by API.
	ReportFileNameSummaryJSON = "/opct-report-summary.json"
	ReportTemplateBasePath    = "data/templates/report"
)

type ReportData struct {
	Summary  *ReportSummary `json:"summary"`
	Raw      string         `json:"-"`
	Provider *ReportResult  `json:"provider"`
	Baseline *ReportResult  `json:"baseline,omitempty"`
	Checks   *ReportChecks  `json:"checks,omitempty"`
	Setup    *ReportSetup   `json:"setup,omitempty"`
}

type ReportChecks struct {
	BaseURL    string       `json:"baseURL"`
	EmptyValue string       `json:"emptyValue"`
	Fail       []*SLOOutput `json:"failures"`
	Pass       []*SLOOutput `json:"successes"`
	Warn       []*SLOOutput `json:"warnings"`
	Skip       []*SLOOutput `json:"skips"`
}

type ReportResult struct {
	Version          *ReportVersion           `json:"version"`
	Infra            *ReportInfra             `json:"infra"`
	ClusterOperators *ReportClusterOperators  `json:"clusterOperators"`
	ClusterHealth    *ReportClusterHealth     `json:"clusterHealth"`
	Plugins          map[string]*ReportPlugin `json:"plugins"`
	HasValidBaseline bool                     `json:"hasValidBaseline"`
	MustGatherInfo   *mustgather.MustGather   `json:"mustGatherInfo,omitempty"`
	ErrorCounters    *archive.ErrorCounter    `json:"errorCounters,omitempty"`
	Runtime          *ReportRuntime           `json:"runtime,omitempty"`
	Nodes            []*summary.Node          `json:"nodes,omitempty"`
}

func (rt *ReportResult) GetPlugins() []string {
	plugins := []string{}
	for pluginName, p := range rt.Plugins {
		if len(p.Name) == 0 {
			log.Debugf("show/terminal: skipping plugin %s", pluginName)
			continue
		}
		plugins = append(plugins, pluginName)
	}
	return plugins
}

type ReportSummary struct {
	Tests    *ReportSummaryTests   `json:"tests"`
	Alerts   *ReportSummaryAlerts  `json:"alerts"`
	Runtime  *ReportSummaryRuntime `json:"runtime,omitempty"`
	Headline string                `json:"headline"`
	Features ReportSummaryFeatures `json:"features,omitempty"`
}

type ReportSummaryFeatures struct {
	HasCAMGI         bool `json:"hasCAMGI,omitempty"`
	HasMetricsData   bool `json:"hasMetricsData,omitempty"`
	HasInstallConfig bool `json:"hasInstallConfig,omitempty"`
}

type ReportSummaryRuntime struct {
	Timers        *metrics.Timers   `json:"timers,omitempty"`
	Plugins       map[string]string `json:"plugins,omitempty"`
	ExecutionTime string            `json:"executionTime,omitempty"`
}

type ReportSummaryTests struct {
	Archive     string `json:"archive"`
	ArchiveDiff string `json:"archiveDiff,omitempty"`
}

type ReportSummaryAlerts struct {
	PluginK8S             string `json:"pluginK8S,omitempty"`
	PluginK8SMessage      string `json:"pluginK8SMessage,omitempty"`
	PluginOCP             string `json:"pluginOCP,omitempty"`
	PluginOCPMessage      string `json:"pluginOCPMessage,omitempty"`
	SuiteErrors           string `json:"suiteErrors,omitempty"`
	SuiteErrorsMessage    string `json:"suiteErrorsMessage,omitempty"`
	WorkloadErrors        string `json:"workloadErrors,omitempty"`
	WorkloadErrorsMessage string `json:"workloadErrorsMessage,omitempty"`
	Checks                string `json:"checks,omitempty"`
	ChecksMessage         string `json:"checksMessage,omitempty"`
}

type ReportVersion struct {
	// OpenShift versions
	OpenShift *summary.SummaryClusterVersionOutput `json:"openshift"`

	// Kubernetes Version
	Kubernetes string `json:"kubernetes"`

	// OPCT Version
	OPCTServer string `json:"opctServer,omitempty"`
	OPCTClient string `json:"opctClient,omitempty"`
}

type ReportInfra struct {
	Name                 string `json:"name"`
	PlatformType         string `json:"platformType"`
	PlatformName         string `json:"platformName"`
	Topology             string `json:"topology,omitempty"`
	ControlPlaneTopology string `json:"controlPlaneTopology,omitempty"`
	APIServerURL         string `json:"apiServerURL,omitempty"`
	APIServerInternalURL string `json:"apiServerInternalURL,omitempty"`
	NetworkType          string `json:"networkType,omitempty"`
}

type ReportClusterOperators struct {
	CountAvailable   uint64 `json:"countAvailable,omitempty"`
	CountProgressing uint64 `json:"countProgressing,omitempty"`
	CountDegraded    uint64 `json:"countDegraded,omitempty"`
}

type ReportClusterHealth struct {
	NodeHealthTotal  int                           `json:"nodeHealthTotal,omitempty"`
	NodeHealthy      int                           `json:"nodeHealthy,omitempty"`
	NodeHealthPerc   float64                       `json:"nodeHealthPerc,omitempty"`
	PodHealthTotal   int                           `json:"podHealthTotal,omitempty"`
	PodHealthy       int                           `json:"podHealthy,omitempty"`
	PodHealthPerc    float64                       `json:"podHealthPerc,omitempty"`
	PodHealthDetails []discovery.HealthInfoDetails `json:"podHealthDetails,omitempty"`
}

type ReportPlugin struct {
	ID            string                       `json:"id"`
	Title         string                       `json:"title"`
	Name          string                       `json:"name"`
	Definition    *plugin.PluginDefinition     `json:"definition,omitempty"`
	Stat          *ReportPluginStat            `json:"stat"`
	ErrorCounters *archive.ErrorCounter        `json:"errorCounters,omitempty"`
	Suite         *summary.OpenshiftTestsSuite `json:"suite"`

	Tests map[string]*plugin.TestItem `json:"tests,omitempty"`

	// Filters
	// SuiteOnly
	FailedFilter1 []*ReportTestFailure `json:"failedTestsFilter1"`
	TagsFilter1   string               `json:"tagsFailuresFilter1"`

	// Filter: BaselineArchive
	FailedFilter2 []*ReportTestFailure `json:"failedTestsFilter2"`
	TagsFilter2   string               `json:"tagsFailuresFilter2"`

	// Filter: FlakeAPI
	FailedFilter3 []*ReportTestFailure `json:"failedTestsFilter3"`
	TagsFilter3   string               `json:"tagsFailuresFilter3"`

	// Filter: BaselineAPI
	FailedFilter4 []*ReportTestFailure `json:"failedTestsFilter4"`
	TagsFilter4   string               `json:"tagsFailuresFilter4"`

	// Filter: KnownFailures
	FailedFilter5 []*ReportTestFailure `json:"failedTestsFilter5"`
	TagsFilter5   string               `json:"tagsFailuresFilter5"`

	// Filter: Replay
	FailedFilter6 []*ReportTestFailure `json:"failedTestsFilter6"`
	TagsFilter6   string               `json:"tagsFailuresFilter6"`

	// Final results after filters
	FailedFiltered []*ReportTestFailure `json:"failedFiltered"`
	TagsFiltered   string               `json:"tagsFailuresFiltered"`
}

func (rp *ReportPlugin) BuildFailedData(filterID string, dataFailures []string) {
	failures := []*ReportTestFailure{}
	tags := plugin.NewTestTagsEmpty(len(dataFailures))
	for _, f := range dataFailures {
		if _, ok := rp.Tests[f]; !ok {
			log.Warnf("BuildFailedData: test %s not found in the plugin", f)
			continue
		}
		// Create a new ReportTestFailure
		rtf := &ReportTestFailure{
			ID:            rp.Tests[f].ID,
			Name:          rp.Tests[f].Name,
			Documentation: rp.Tests[f].Documentation,
		}
		if rp.Tests[f].Flake != nil {
			rtf.FlakeCount = rp.Tests[f].Flake.CurrentFlakes
			rtf.FlakePerc = rp.Tests[f].Flake.CurrentFlakePerc
		}
		if _, ok := rp.Tests[f].ErrorCounters["total"]; ok {
			rtf.ErrorsCount = int64(rp.Tests[f].ErrorCounters["total"])
		}
		tags.Add(&f)
		failures = append(failures, rtf)
	}
	failures = sortReportTestFailure(failures)
	switch filterID {
	case "final":
		rp.FailedFiltered = failures
		rp.TagsFiltered = tags.ShowSorted()
	case "F1":
		rp.FailedFilter1 = failures
		rp.TagsFilter1 = tags.ShowSorted()
	case "F3":
		rp.FailedFilter3 = failures
		rp.TagsFilter3 = tags.ShowSorted()
	case "F4":
		rp.FailedFilter4 = failures
		rp.TagsFilter4 = tags.ShowSorted()
	case "F5":
		rp.FailedFilter5 = failures
		rp.TagsFilter5 = tags.ShowSorted()
	case "F6":
		rp.FailedFilter6 = failures
		rp.TagsFilter6 = tags.ShowSorted()
	}
}

type ReportPluginStat struct {
	Completed string `json:"execution"`
	Result    string `json:"result"`
	Status    string `json:"status"`
	Total     int64  `json:"total"`
	Passed    int64  `json:"passed"`
	Failed    int64  `json:"failed"`
	Timeout   int64  `json:"timeout"`
	Skipped   int64  `json:"skipped"`

	// Filters
	// Filter: SuiteOnly
	FilterSuite     int64 `json:"filter1Suite"`
	Filter1Excluded int64 `json:"filter1Excluded"`

	// Filter: BaselineArchive (deprecated soon)
	FilterBaseline  int64 `json:"filter2Baseline"`
	Filter2Excluded int64 `json:"filter2Excluded"`

	// Filter: FlakeCI
	FilterFailedPrio int64 `json:"filter3FailedPriority"`
	Filter3Excluded  int64 `json:"filter3Excluded"`

	// Filter: BaselineAPI
	FilterFailedAPI int64 `json:"filter4FailedAPI"`
	Filter4Excluded int64 `json:"filter4Excluded"`

	// Filter: KnownFailures
	Filter5Failures int64 `json:"filter5Failures"`
	Filter5Excluded int64 `json:"filter5Excluded"`

	// Filter: Replay
	Filter6Failures int64 `json:"filter6Failures"`
	Filter6Excluded int64 `json:"filter6Excluded"`

	FilterFailures int64 `json:"filterFailures"`
}

type ReportTestFailure struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Documentation string  `json:"documentation"`
	FlakePerc     float64 `json:"flakePerc"`
	FlakeCount    int64   `json:"flakeCount"`
	ErrorsCount   int64   `json:"errorsTotal"`
}

type ReportSetup struct {
	Frontend *ReportSetupFrontend `json:"frontend,omitempty"`
	API      *ReportSetupAPI      `json:"api,omitempty"`
}
type ReportSetupFrontend struct {
	EmbedData bool
}

type ReportSetupAPI struct {
	SummaryName      string `json:"dataPath,omitempty"`
	SummaryArchive   string `json:"summaryArchive,omitempty"`
	UUID             string `json:"uuid,omitempty"`
	ExecutionDate    string `json:"executionDate,omitempty"`
	OpenShiftVersion string `json:"openshiftVersion,omitempty"`
	OpenShiftRelease string `json:"openshiftRelease,omitempty"`
	PlatformType     string `json:"platformType,omitempty"`
	ProviderName     string `json:"providerName,omitempty"`
	InfraTopology    string `json:"infraTopology,omitempty"`
	Workflow         string `json:"workflow,omitempty"`
}

type ReportRuntime struct {
	ServerLogs   []*archive.RuntimeInfoItem `json:"serverLogs,omitempty"`
	ServerConfig []*archive.RuntimeInfoItem `json:"serverConfig,omitempty"`
	OpctConfig   []*archive.RuntimeInfoItem `json:"opctConfig,omitempty"`
}

func NewReportData(embedFrontend bool) *ReportData {
	return &ReportData{
		Provider: &ReportResult{},
		Setup: &ReportSetup{
			Frontend: &ReportSetupFrontend{
				EmbedData: embedFrontend,
			},
			API: &ReportSetupAPI{},
		},
	}
}

// Populate is a entrypoint to initialize, trigger the data source processors,
// and finalize the report data structure used by frontend (HTML or CLI).
func (re *ReportData) Populate(cs *summary.ConsolidatedSummary) error {
	cs.Timers.Add("report-populate")
	re.Summary = &ReportSummary{
		Tests: &ReportSummaryTests{
			Archive: cs.GetProvider().Archive,
		},
		Runtime: &ReportSummaryRuntime{
			Plugins: make(map[string]string, 4),
		},
		Alerts: &ReportSummaryAlerts{},
	}
	if err := re.populateSource(cs.GetProvider()); err != nil {
		return err
	}
	re.Provider.HasValidBaseline = cs.GetBaseline().HasValidResults()
	if re.Provider.HasValidBaseline {
		if err := re.populateSource(cs.GetBaseline()); err != nil {
			return err
		}
		re.Summary.Tests.ArchiveDiff = cs.GetBaseline().Archive
		re.Summary.Headline = fmt.Sprintf("%s (diff %s) | OCP %s | K8S %s",
			re.Summary.Tests.Archive,
			re.Summary.Tests.ArchiveDiff,
			re.Provider.Version.OpenShift.Desired,
			re.Provider.Version.Kubernetes,
		)
	}

	re.Summary.Features = ReportSummaryFeatures{
		HasCAMGI:         cs.Provider.HasCAMGI,
		HasMetricsData:   cs.Provider.HasMetrics,
		HasInstallConfig: cs.Provider.HasInstallConfig,
	}

	// Checks need to run after the report is populated, so it can evaluate the
	// data entirely.
	checks := NewCheckSummary(re)
	if err := checks.Run(); err != nil {
		log.Debugf("one or more errors found when running checks: %v", err)
	}
	pass, fail, warn, skip := checks.GetCheckResults()
	re.Checks = &ReportChecks{
		BaseURL:    checks.GetBaseURL(),
		EmptyValue: CheckIdEmptyValue,
		Pass:       pass,
		Fail:       fail,
		Warn:       warn,
		Skip:       skip,
	}
	if len(re.Checks.Fail) > 0 {
		re.Summary.Alerts.Checks = "danger"
		re.Summary.Alerts.ChecksMessage = fmt.Sprintf("%d", len(re.Checks.Fail))
	}
	cs.Timers.Add("report-populate")
	re.Summary.Runtime.Timers = cs.Timers
	return nil
}

// populateSource reads the loaded data, creating a report data for each result
// data source (provider and/or baseline).
func (re *ReportData) populateSource(rs *summary.ResultSummary) error {
	var reResult *ReportResult
	if rs.Name == summary.ResultSourceNameBaseline {
		re.Baseline = &ReportResult{}
		reResult = re.Baseline
	} else {
		re.Provider = &ReportResult{}
		reResult = re.Provider
		reResult.MustGatherInfo = rs.MustGather
	}
	// Version
	v, err := rs.GetOpenShift().GetClusterVersion()
	if err != nil {
		return err
	}
	reResult.Version = &ReportVersion{
		OpenShift:  v,
		Kubernetes: rs.GetSonobuoyCluster().APIVersion,
	}

	// Infrastructure
	infra, err := rs.GetOpenShift().GetInfrastructure()
	if err != nil {
		return err
	}
	platformName := ""
	if string(infra.Status.PlatformStatus.Type) == "External" {
		platformName = infra.Spec.PlatformSpec.External.PlatformName
	}
	sdn, err := rs.GetOpenShift().GetClusterNetwork()
	if err != nil {
		log.Errorf("unable to get clusterNetwork object: %v", err)
		return err
	}
	reResult.Infra = &ReportInfra{
		PlatformType:         string(infra.Status.PlatformStatus.Type),
		PlatformName:         platformName,
		Name:                 string(infra.Status.InfrastructureName),
		Topology:             string(infra.Status.InfrastructureTopology),
		ControlPlaneTopology: string(infra.Status.ControlPlaneTopology),
		APIServerURL:         string(infra.Status.APIServerURL),
		APIServerInternalURL: string(infra.Status.APIServerInternalURL),
		NetworkType:          string(sdn.Spec.NetworkType),
	}

	// Cluster Operators
	co, err := rs.GetOpenShift().GetClusterOperator()
	if err != nil {
		return err
	}
	reResult.ClusterOperators = &ReportClusterOperators{
		CountAvailable:   co.CountAvailable,
		CountProgressing: co.CountProgressing,
		CountDegraded:    co.CountDegraded,
	}

	// Node
	reResult.Nodes = rs.GetOpenShift().GetNodes()

	// Node and Pod Status
	sbCluster := rs.GetSonobuoyCluster()
	reResult.ClusterHealth = &ReportClusterHealth{
		NodeHealthTotal: sbCluster.NodeHealth.Total,
		NodeHealthy:     sbCluster.NodeHealth.Healthy,
		NodeHealthPerc:  float64(100 * sbCluster.NodeHealth.Healthy / sbCluster.NodeHealth.Total),
		PodHealthTotal:  sbCluster.PodHealth.Total,
		PodHealthy:      sbCluster.PodHealth.Healthy,
		PodHealthPerc:   float64(100 * sbCluster.PodHealth.Healthy / sbCluster.PodHealth.Total),
	}
	for _, dt := range sbCluster.PodHealth.Details {
		if !dt.Healthy {
			reResult.ClusterHealth.PodHealthDetails = append(reResult.ClusterHealth.PodHealthDetails, dt)
		}
	}

	// Populate plugins. New plgins must be added here.
	availablePlugins := []string{
		plugin.PluginNameOpenShiftUpgrade,
		plugin.PluginNameKubernetesConformance,
		plugin.PluginNameOpenShiftConformance,
		plugin.PluginNameConformanceReplay,
		plugin.PluginNameArtifactsCollector,
	}
	reResult.Plugins = make(map[string]*ReportPlugin, len(availablePlugins))
	for _, pluginID := range availablePlugins {
		if err := re.populatePluginConformance(rs, reResult, pluginID); err != nil {
			return err
		}
	}

	// Aggregate Plugin errors
	reResult.ErrorCounters = archive.MergeErrorCounters(
		reResult.Plugins[plugin.PluginNameKubernetesConformance].ErrorCounters,
		reResult.Plugins[plugin.PluginNameOpenShiftConformance].ErrorCounters,
	)

	// Runtime
	if reResult.Runtime == nil {
		reResult.Runtime = &ReportRuntime{}
	}
	var serverFinishedTime string
	if rs.Sonobuoy != nil && rs.Sonobuoy.MetaRuntime != nil {
		reResult.Runtime.ServerLogs = rs.Sonobuoy.MetaRuntime
		for _, e := range rs.Sonobuoy.MetaRuntime {
			if strings.HasPrefix(e.Name, "plugin finished") {
				arr := strings.Split(e.Name, "plugin finished ")
				re.Summary.Runtime.Plugins[arr[len(arr)-1]] = e.Delta
			}
			if strings.HasPrefix(e.Name, "server finished") {
				re.Summary.Runtime.ExecutionTime = e.Total
				serverFinishedTime = e.Time
			}
		}
	}
	if rs.Sonobuoy != nil && rs.Sonobuoy.MetaConfig != nil {
		reResult.Runtime.ServerConfig = rs.Sonobuoy.MetaConfig
	}
	if rs.Sonobuoy != nil && rs.Sonobuoy.MetaConfig != nil {
		reResult.Runtime.OpctConfig = rs.Sonobuoy.OpctConfig
	}

	// Setup/API data: Copy relevant data to me used as metadata
	// of archive in the API.
	if re.Setup == nil {
		re.Setup = &ReportSetup{}
	}
	if re.Setup.API == nil {
		re.Setup.API = &ReportSetupAPI{}
	}
	re.Setup.API.InfraTopology = reResult.Infra.Topology
	re.Setup.API.PlatformType = string(infra.Status.PlatformStatus.Type)
	re.Setup.API.ProviderName = string(infra.Status.PlatformStatus.Type)
	if platformName != "" {
		re.Setup.API.ProviderName = platformName
	}
	// Setup/API data: OpenShift version
	ocpVersion := reResult.Version.OpenShift.Desired
	re.Setup.API.OpenShiftVersion = ocpVersion
	re.Setup.API.OpenShiftRelease = fmt.Sprintf("%s.%s", strings.Split(ocpVersion, ".")[0], strings.Split(ocpVersion, ".")[1])

	// Discover execution time
	re.Setup.API.ExecutionDate = serverFinishedTime
	if serverFinishedTime != "" {
		ts := strings.Replace(serverFinishedTime, "-", "", -1)
		ts = strings.Replace(ts, ":", "", -1)
		ts = strings.Replace(ts, "T", "", -1)
		ts = strings.Replace(ts, "Z", "", -1)
		re.Setup.API.SummaryName = fmt.Sprintf("%s_%s_%s.json", re.Setup.API.OpenShiftRelease, re.Setup.API.PlatformType, ts)
	}
	for i := range reResult.Runtime.ServerConfig {
		if reResult.Runtime.ServerConfig[i].Name == "UUID" {
			re.Setup.API.UUID = reResult.Runtime.ServerConfig[i].Value
		}
	}
	for i := range reResult.Runtime.OpctConfig {
		if reResult.Runtime.OpctConfig[i].Name == "run-mode" {
			re.Setup.API.Workflow = reResult.Runtime.ServerConfig[i].Value
		}
	}
	return nil
}

// populatePluginConformance reads the plugin data, processing and creating the report data.
func (re *ReportData) populatePluginConformance(rs *summary.ResultSummary, reResult *ReportResult, pluginID string) error {
	var pluginSum *plugin.OPCTPluginSummary
	var suite *summary.OpenshiftTestsSuite
	var pluginTitle string
	var pluginAlert string
	var pluginAlertMessage string

	switch pluginID {
	case plugin.PluginNameKubernetesConformance:
		pluginSum = rs.GetOpenShift().GetResultK8SValidated()
		pluginTitle = "Results for Kubernetes Conformance Suite"
		suite = rs.GetSuites().KubernetesConformance
	case plugin.PluginNameOpenShiftConformance:
		pluginSum = rs.GetOpenShift().GetResultOCPValidated()
		pluginTitle = "Results for OpenShift Conformance Suite"
		suite = rs.GetSuites().OpenshiftConformance
	case plugin.PluginNameOpenShiftUpgrade:
		pluginSum = rs.GetOpenShift().GetResultConformanceUpgrade()
		pluginTitle = "Results for OpenShift Conformance Upgrade Suite"
	case plugin.PluginNameConformanceReplay:
		pluginSum = rs.GetOpenShift().GetResultConformanceReplay()
		pluginTitle = "Results for Replay test suite"
	case plugin.PluginNameArtifactsCollector:
		pluginSum = rs.GetOpenShift().GetResultArtifactsCollector()
		pluginTitle = "Results for Plugin Collector"
	}

	pluginRes := pluginSum.Status
	reResult.Plugins[pluginID] = &ReportPlugin{
		ID:    pluginID,
		Title: pluginTitle,
		Name:  pluginSum.Name,
		Stat: &ReportPluginStat{
			Completed: "TODO",
			Status:    pluginSum.Status,
			Result:    pluginRes,
			Total:     pluginSum.Total,
			Passed:    pluginSum.Passed,
			Failed:    pluginSum.Failed,
			Timeout:   pluginSum.Timeout,
			Skipped:   pluginSum.Skipped,
		},
		Suite: suite,
		Tests: pluginSum.Tests,
	}

	// No more advanced fields to create for non-Conformance
	switch pluginID {
	case plugin.PluginNameOpenShiftUpgrade, plugin.PluginNameArtifactsCollector:
		return nil
	}

	// Set counters for each filters in the pipeline
	// Filter SuiteOnly
	reResult.Plugins[pluginID].Stat.FilterSuite = int64(len(pluginSum.FailedFilter1))
	reResult.Plugins[pluginID].Stat.Filter1Excluded = int64(len(pluginSum.FailedExcludedFilter1))

	// Filter Baseline
	reResult.Plugins[pluginID].Stat.FilterBaseline = int64(len(pluginSum.FailedFilter2))
	reResult.Plugins[pluginID].Stat.Filter2Excluded = int64(len(pluginSum.FailedExcludedFilter2))

	// Filter FlakeAPI
	reResult.Plugins[pluginID].Stat.FilterFailedPrio = int64(len(pluginSum.FailedFilter3))
	reResult.Plugins[pluginID].Stat.Filter3Excluded = int64(len(pluginSum.FailedExcludedFilter3))

	// Filter BaselineAPI
	reResult.Plugins[pluginID].Stat.FilterFailedAPI = int64(len(pluginSum.FailedFilter4))
	reResult.Plugins[pluginID].Stat.Filter4Excluded = int64(len(pluginSum.FailedExcludedFilter4))

	// Filter KnownFailures
	reResult.Plugins[pluginID].Stat.Filter5Failures = int64(len(pluginSum.FailedFilter5))
	reResult.Plugins[pluginID].Stat.Filter5Excluded = int64(len(pluginSum.FailedExcludedFilter5))

	// Filter Replay
	reResult.Plugins[pluginID].Stat.Filter6Failures = int64(len(pluginSum.FailedFilter6))
	reResult.Plugins[pluginID].Stat.Filter6Excluded = int64(len(pluginSum.FailedExcludedFilter6))

	// Filter Failures (result)
	reResult.Plugins[pluginID].Stat.FilterFailures = int64(len(pluginSum.FailedFiltered))
	reResult.Plugins[pluginID].ErrorCounters = pluginSum.GetErrorCounters()

	// Will consider passed when all conformance tests have passed (removing monitor)
	hasRuntimeError := (reResult.Plugins[pluginID].Stat.Total == 1) && (reResult.Plugins[pluginID].Stat.Failed == 1)
	if !hasRuntimeError {
		if reResult.Plugins[pluginID].Stat.FilterFailures == 0 {
			reResult.Plugins[pluginID].Stat.Result = "passed"
		}
		// Replay is a special case, it can have failures in the filter6 as it is
		// a replay of the failures from original suite which can have perm failures or bugs.
		// Replay helps in debugging and getting more confidence in the results.
		if pluginID == plugin.PluginNameConformanceReplay && reResult.Plugins[pluginID].Stat.Filter6Failures != 0 {
			reResult.Plugins[pluginID].Stat.Result = "---"
		}
	}

	if reResult.Plugins[pluginID].Stat.FilterFailures != 0 {
		pluginAlert = "danger"
		pluginAlertMessage = fmt.Sprintf("%d", int64(len(pluginSum.FailedFiltered)))
	} else if reResult.Plugins[pluginID].Stat.FilterSuite != 0 {
		pluginAlert = "warning"
		pluginAlertMessage = fmt.Sprintf("%d", int64(len(pluginSum.FailedFilter1)))
	}

	if _, ok := rs.GetSonobuoy().PluginsDefinition[pluginID]; ok {
		def := rs.GetSonobuoy().PluginsDefinition[pluginID]
		reResult.Plugins[pluginID].Definition = &plugin.PluginDefinition{
			PluginImage:   def.Definition.Spec.Image,
			SonobuoyImage: def.SonobuoyImage,
			Name:          def.Definition.SonobuoyConfig.PluginName,
		}
	}

	// Filter failures
	// Final filters (results/priority)
	reResult.Plugins[pluginID].BuildFailedData("final", pluginSum.FailedFiltered)

	// Filter flakeAPI
	reResult.Plugins[pluginID].BuildFailedData("F3", pluginSum.FailedExcludedFilter3)

	// Filter BaselineAPI
	reResult.Plugins[pluginID].BuildFailedData("F4", pluginSum.FailedExcludedFilter4)

	// Filter Replay
	reResult.Plugins[pluginID].BuildFailedData("F6", pluginSum.FailedExcludedFilter6)

	// Filter KnownFailures
	reResult.Plugins[pluginID].BuildFailedData("F5", pluginSum.FailedExcludedFilter5)

	// Filter SuiteOnly
	reResult.Plugins[pluginID].BuildFailedData("F1", pluginSum.FailedExcludedFilter1)

	// update alerts
	if rs.Name == summary.ResultSourceNameProvider && pluginAlert != "" {
		switch pluginID {
		case plugin.PluginNameKubernetesConformance:
			re.Summary.Alerts.PluginK8S = pluginAlert
			re.Summary.Alerts.PluginK8SMessage = pluginAlertMessage
		case plugin.PluginNameOpenShiftConformance:
			re.Summary.Alerts.PluginOCP = pluginAlert
			re.Summary.Alerts.PluginOCPMessage = pluginAlertMessage
		}
	}

	return nil
}

// SaveResults persist the processed data to the result directory.
func (re *ReportData) SaveResults(path string) error {
	re.Summary.Runtime.Timers.Add("report-save/results")

	// opct-report.json (data source)
	reportData, err := json.MarshalIndent(re, "", " ")
	if err != nil {
		return fmt.Errorf("unable to process report data/report.json: %v", err)
	}
	// used when not using http file server
	if re.Setup.Frontend.EmbedData {
		re.Raw = string(reportData)
	}

	// save the report data to the result directory
	err = os.WriteFile(fmt.Sprintf("%s/%s", path, ReportFileNameIndexJSON), reportData, 0644)
	if err != nil {
		return fmt.Errorf("unable to save report data/report.json: %v", err)
	}

	// create a summarized JSON to be used as baseline.
	// reSummary, err := re.CopySummary()
	var reSummary ReportData
	skipSummary := false
	if err := re.DeepCopyInto(&reSummary); err != nil {
		log.Errorf("unable to copy report summary: %v", err)
		skipSummary = true
	}
	// clean up the report data for summary artifact.
	if !skipSummary {
		if err := reSummary.SummaryBuilder(); err != nil {
			log.Errorf("unable to build report summary: %v", err)
			skipSummary = true
		}
	}
	// Serialize the report summary data to JSON.
	if !skipSummary {
		reSummaryData, err := json.MarshalIndent(reSummary, "", " ")
		if err != nil {
			log.Errorf("unable to marshal report summary data: %v", err)
		} else {
			// save the report summary data to the result directory
			err = os.WriteFile(fmt.Sprintf("%s/%s", path, ReportFileNameSummaryJSON), reSummaryData, 0644)
			if err != nil {
				log.Errorf("unable to marshal report summary data: %v", err)
			}
		}
	}

	// render the template files from frontend report pages.
	for _, file := range []string{"report.html", "report.css", "filter.html"} {
		log.Debugf("Processing file %s\n", file)
		srcTemplate := fmt.Sprintf("%s/%s", ReportTemplateBasePath, file)
		destFile := fmt.Sprintf("%s/opct-%s", path, file)
		if file == "report.html" {
			destFile = fmt.Sprintf("%s/index.html", path)
		}

		datS, err := vfs.GetData().ReadFile(srcTemplate)
		if err != nil {
			return fmt.Errorf("unable to read file %q from VFS: %v", srcTemplate, err)
		}

		// Change Go template delimiter to '[[]]' preventing conflict with
		// javascript delimiter '{{}}' in the frontend.
		tmplS, err := template.New("report").Delims("[[", "]]").Parse(string(datS))
		if err != nil {
			return fmt.Errorf("unable to create template for %q: %v", srcTemplate, err)
		}

		var fileBufferS bytes.Buffer
		err = tmplS.Execute(&fileBufferS, re)
		if err != nil {
			return fmt.Errorf("unable to process template for %q: %v", srcTemplate, err)
		}

		err = os.WriteFile(destFile, fileBufferS.Bytes(), 0644)
		if err != nil {
			return fmt.Errorf("unable to save %q: %v", srcTemplate, err)
		}
	}

	re.Summary.Runtime.Timers.Add("report-save/results")
	return nil
}

// ShowJSON print the raw json in stdout.
func (re *ReportData) ShowJSON() (string, error) {
	val, err := json.MarshalIndent(re, "", "    ")
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// DeepCopy creates a deep copy of the report data.
// The function uses the json.Marshal and json.Unmarshal to create a new copy of the data
// without any reference to the original data.
func (re *ReportData) DeepCopyInto(newRe *ReportData) error {
	// var newReport ReportData
	newReportData, err := json.Marshal(re)
	if err != nil {
		return err
	}
	err = json.Unmarshal(newReportData, &newRe)
	if err != nil {
		return err
	}
	return nil
}

func (re *ReportData) SummaryBuilder() error {
	// Clean up success tests for each plugin.
	for p := range re.Provider.Plugins {
		re.Provider.Plugins[p].Tests = nil
	}
	// Cleaning useless data from etcd logs parser
	if re.Provider != nil &&
		re.Provider.MustGatherInfo != nil {
		if re.Provider.MustGatherInfo.ErrorEtcdLogs != nil {

			for k := range re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll {
				re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll[k].StatOutliers = ""
			}
			re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowHour = nil
		}
		re.Provider.MustGatherInfo.NamespaceErrors = nil
		re.Provider.MustGatherInfo.PodNetworkChecks.Checks = nil
	}
	// What else to clean up?
	return nil
}

//
// Sorting functions
//

// SortedTestFailure stores the key/value to rank by Key.
type SortedTestFailure struct {
	Key   *ReportTestFailure
	Value int
}

func sortReportTestFailure(items []*ReportTestFailure) []*ReportTestFailure {
	rank := make(SortedListTestFailure, len(items))
	i := 0
	for _, v := range items {
		rank[i] = SortedTestFailure{v, int(v.ErrorsCount)}
		i++
	}
	sort.Sort(sort.Reverse(rank))
	newItems := make([]*ReportTestFailure, len(items))
	for i, data := range rank {
		newItems[i] = data.Key
	}
	return newItems
}

// SortedList stores the list of key/value map, implementing interfaces
// to sort/rank a map strings with integers as values.
type SortedListTestFailure []SortedTestFailure

func (p SortedListTestFailure) Len() int           { return len(p) }
func (p SortedListTestFailure) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p SortedListTestFailure) Less(i, j int) bool { return p[i].Value < p[j].Value }
