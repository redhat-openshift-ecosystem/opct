package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"

	vfs "github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/assets"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/archive"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/metrics"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/plugin"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/summary"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/openshift/mustgather"
	log "github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/sonobuoy/pkg/discovery"
)

const (
	ReportFileNameIndexJSON = "/opct-report.json"
	ReportTemplateBasePath  = "data/templates/report"
)

type Report struct {
	Summary  *ReportSummary `json:"summary"`
	Raw      string         `json:"-"`
	Provider *ReportResult  `json:"provider"`
	Baseline *ReportResult  `json:"baseline,omitempty"`
	Checks   *ReportChecks  `json:"checks,omitempty"`
	Setup    *ReportSetup
}

type ReportChecks struct {
	BaseURL string   `json:"baseURL"`
	Fail    []*Check `json:"failures"`
	Pass    []*Check `json:"successes"`
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
}

type ReportSummary struct {
	Tests    *ReportSummaryTests   `json:"tests"`
	Alerts   *ReportSummaryAlerts  `json:"alerts"`
	Runtime  *ReportSummaryRuntime `json:"runtime,omitempty"`
	Headline string                `json:"headline"`
}

type ReportSummaryRuntime struct {
	Timers        metrics.Timers    `json:"timers,omitempty"`
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
	ID            string                   `json:"id"`
	Title         string                   `json:"title"`
	Name          string                   `json:"name"`
	Definition    *plugin.PluginDefinition `json:"definition,omitempty"`
	Stat          *ReportPluginStat        `json:"stat"`
	ErrorCounters *archive.ErrorCounter    `json:"errorCounters,omitempty"`
	// CountFilterSuite uint64               `json:"countFilterSuite"`
	// CountFilterBase  uint64               `json:"countFilterBase"`
	// CountFilterPrio  uint64               `json:"countFilterFilterPrio"`
	Suite           *summary.OpenshiftTestsSuite `json:"suite"`
	TagsFailedPrio  string                       `json:"tagsFailuresPriority"`
	TestsFailedPrio []*ReportTestFailure         `json:"testsFailuresPriority"`
	TagsFlakeCI     string                       `json:"tagsFlakeCI"`
	TestsFlakeCI    []*ReportTestFailure         `json:"testsFlakeCI"`
	Tests           map[string]*plugin.TestItem  `json:"tests,omitempty"`
}

type ReportPluginStat struct {
	Completed        string `json:"execution"`
	Result           string `json:"result"`
	Status           string `json:"status"`
	Total            int64  `json:"total"`
	Passed           int64  `json:"passed"`
	Failed           int64  `json:"failed"`
	Timeout          int64  `json:"timeout"`
	Skipped          int64  `json:"skipped"`
	FilterSuite      int64  `json:"filterSuite"`
	FilterBaseline   int64  `json:"filterBaseline"`
	FilterFailedPrio int64  `json:"filterFailedPriority"`
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
	Frontend *ReportSetupFrontend
}
type ReportSetupFrontend struct {
	EmbedData bool
}

type ReportRuntime struct {
	ServerLogs   []*archive.RuntimeInfoItem `json:"serverLogs,omitempty"`
	ServerConfig []*archive.RuntimeInfoItem `json:"serverConfig,omitempty"`
	OpctConfig   []*archive.RuntimeInfoItem `json:"opctConfig,omitempty"`
}

func (re *Report) Populate(cs *summary.ConsolidatedSummary) error {
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
	checks := NewCheckSummary(re)
	err := checks.Run()
	if err != nil {
		log.Debugf("one or more errors found when running checks: %v", err)
	}
	re.Checks = &ReportChecks{
		BaseURL: checks.GetBaseURL(),
		Pass:    checks.GetChecksPassed(),
		Fail:    checks.GetChecksFailed(),
	}
	if len(re.Checks.Fail) > 0 {
		re.Summary.Alerts.Checks = "danger"
		re.Summary.Alerts.ChecksMessage = fmt.Sprintf("%d", len(re.Checks.Fail))
	}

	cs.Timers.Add("report-populate")
	re.Summary.Runtime.Timers = cs.Timers
	return nil
}

func (re *Report) populateSource(rs *summary.ResultSummary) error {

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
	partnerPlatformName := string(infra.Status.PlatformStatus.Type)
	if partnerPlatformName == "External" {
		partnerPlatformName = fmt.Sprintf("%s (%s)", partnerPlatformName, infra.Spec.PlatformSpec.External.PlatformName)
	}
	sdn, err := rs.GetOpenShift().GetClusterNetwork()
	if err != nil {
		log.Errorf("unable to get clusterNetwork object: %v", err)
		return err
	}
	reResult.Infra = &ReportInfra{
		PlatformType:         partnerPlatformName,
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

	// Plugins
	reResult.Plugins = make(map[string]*ReportPlugin, 4)
	if err := re.populatePluginConformance(rs, reResult, plugin.PluginNameKubernetesConformance); err != nil {
		return err
	}
	if err := re.populatePluginConformance(rs, reResult, plugin.PluginNameOpenShiftConformance); err != nil {
		return err
	}
	if err := re.populatePluginConformance(rs, reResult, plugin.PluginNameOpenShiftUpgrade); err != nil {
		return err
	}
	if err := re.populatePluginConformance(rs, reResult, plugin.PluginNameArtifactsCollector); err != nil {
		return err
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
	if rs.Sonobuoy != nil && rs.Sonobuoy.MetaRuntime != nil {
		reResult.Runtime.ServerLogs = rs.Sonobuoy.MetaRuntime
		for _, e := range rs.Sonobuoy.MetaRuntime {
			if strings.HasPrefix(e.Name, "plugin finished") {
				arr := strings.Split(e.Name, "plugin finished ")
				re.Summary.Runtime.Plugins[arr[len(arr)-1]] = e.Delta
			}
			if strings.HasPrefix(e.Name, "server finished") {
				re.Summary.Runtime.ExecutionTime = e.Total
			}
		}
	}
	if rs.Sonobuoy != nil && rs.Sonobuoy.MetaConfig != nil {
		reResult.Runtime.ServerConfig = rs.Sonobuoy.MetaConfig
	}
	if rs.Sonobuoy != nil && rs.Sonobuoy.MetaConfig != nil {
		reResult.Runtime.OpctConfig = rs.Sonobuoy.OpctConfig
	}
	return nil
}

func (re *Report) populatePluginConformance(rs *summary.ResultSummary, reResult *ReportResult, pluginID string) error {

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
			// FilterSuite:      int64(len(plugin.FailedFilterSuite)),
			// FilterBaseline:   int64(len(plugin.FailedFilterBaseline)),
			// FilterFailedPrio: int64(len(plugin.FailedFilterPrio)),
		},
		Suite: suite,
		Tests: pluginSum.Tests,
		// ErrorCounters: plugin.GetErrorCounters(),
	}

	// No more advanced fields to create for non-Conformance
	switch pluginID {
	case plugin.PluginNameOpenShiftUpgrade, plugin.PluginNameArtifactsCollector:
		return nil
	}

	// Fill stat for filters (non-standard in Sonobuoy)
	reResult.Plugins[pluginID].Stat.FilterSuite = int64(len(pluginSum.FailedFilterSuite))
	reResult.Plugins[pluginID].Stat.FilterBaseline = int64(len(pluginSum.FailedFilterBaseline))
	reResult.Plugins[pluginID].Stat.FilterFailedPrio = int64(len(pluginSum.FailedFilterPrio))
	reResult.Plugins[pluginID].ErrorCounters = pluginSum.GetErrorCounters()
	// Will consider passed when all conformance tests have passed (removing monitor)
	if reResult.Plugins[pluginID].Stat.FilterSuite == 0 {
		reResult.Plugins[pluginID].Stat.Result = "passed"
	}

	if reResult.Plugins[pluginID].Stat.FilterFailedPrio != 0 {
		pluginAlert = "danger"
		pluginAlertMessage = fmt.Sprintf("%d", int64(len(pluginSum.FailedFilterPrio)))
	} else if reResult.Plugins[pluginID].Stat.FilterSuite != 0 {
		pluginAlert = "warning"
		pluginAlertMessage = fmt.Sprintf("%d", int64(len(pluginSum.FailedFilterSuite)))
	}

	if _, ok := rs.GetSonobuoy().PluginsDefinition[pluginID]; ok {
		def := rs.GetSonobuoy().PluginsDefinition[pluginID]
		reResult.Plugins[pluginID].Definition = &plugin.PluginDefinition{
			PluginImage:   def.Definition.Spec.Image,
			SonobuoyImage: def.SonobuoyImage,
			Name:          def.Definition.SonobuoyConfig.PluginName,
		}
	}

	// TODO move this filter to a dedicated function
	noFlakes := make(map[string]struct{})
	testTagsFailedPrio := plugin.NewTestTagsEmpty(len(pluginSum.FailedFilterPrio))
	for _, test := range pluginSum.FailedFilterPrio {
		noFlakes[test] = struct{}{}
		testTagsFailedPrio.Add(&test)
		testData := &ReportTestFailure{
			Name:          test,
			ID:            pluginSum.Tests[test].ID,
			Documentation: pluginSum.Tests[test].Documentation,
		}
		if _, ok := pluginSum.Tests[test].ErrorCounters["total"]; ok {
			testData.ErrorsCount = int64(pluginSum.Tests[test].ErrorCounters["total"])
		}
		reResult.Plugins[pluginID].TestsFailedPrio = append(reResult.Plugins[pluginID].TestsFailedPrio, testData)
	}
	reResult.Plugins[pluginID].TagsFailedPrio = testTagsFailedPrio.ShowSorted()
	reResult.Plugins[pluginID].TestsFailedPrio = sortReportTestFailure(reResult.Plugins[pluginID].TestsFailedPrio)

	flakes := reResult.Plugins[pluginID].TestsFlakeCI
	testTagsFlakeCI := plugin.NewTestTagsEmpty(len(pluginSum.FailedFilterBaseline))
	for _, test := range pluginSum.FailedFilterBaseline {
		if _, ok := noFlakes[test]; ok {
			continue
		}
		testData := &ReportTestFailure{Name: test, ID: pluginSum.Tests[test].ID}
		if pluginSum.Tests[test].Flake != nil {
			testData.FlakeCount = pluginSum.Tests[test].Flake.CurrentFlakes
			testData.FlakePerc = pluginSum.Tests[test].Flake.CurrentFlakePerc
		}
		testTagsFlakeCI.Add(&test)
		if _, ok := pluginSum.Tests[test].ErrorCounters["total"]; ok {
			testData.ErrorsCount = int64(pluginSum.Tests[test].ErrorCounters["total"])
		}
		flakes = append(flakes, testData)
	}
	reResult.Plugins[pluginID].TestsFlakeCI = sortReportTestFailure(flakes)
	reResult.Plugins[pluginID].TagsFlakeCI = testTagsFlakeCI.ShowSorted()

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

func (re *Report) SaveResults(path string) error {
	re.Summary.Runtime.Timers.Add("report-save/results")

	// opct-report.json (data source)
	reportData, err := json.MarshalIndent(re, "", " ")
	check(err)
	// optional, but used when not using http file server
	re.Raw = string(reportData)

	err = os.WriteFile(fmt.Sprintf("%s/%s", path, ReportFileNameIndexJSON), reportData, 0644)
	check(err)

	// static files
	for _, file := range []string{"report.html", "filter.html"} {
		srcTemplate := fmt.Sprintf("%s/%s", ReportTemplateBasePath, file)
		destFile := fmt.Sprintf("%s/opct-%s", path, file)

		datS, err := vfs.GetData().ReadFile(srcTemplate)
		check(err)

		tmplS, err := template.New("report").Delims("[[", "]]").Parse(string(datS))
		check(err)

		var fileBufferS bytes.Buffer
		err = tmplS.Execute(&fileBufferS, re)
		check(err)

		err = os.WriteFile(destFile, fileBufferS.Bytes(), 0644)
		check(err)
	}

	re.Summary.Runtime.Timers.Add("report-save/results")
	return nil
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func (re *Report) ShowJSON() (string, error) {
	val, err := json.MarshalIndent(re, "", "    ")
	if err != nil {
		return "", err
	}
	return string(val), nil
}

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
