package report

import (
	"bytes"
	"html/template"
	"os"
	"strings"
	"testing"

	efs "github.com/redhat-openshift-ecosystem/opct/internal/assets"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/archive"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/metrics"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/summary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/manifest"
	v1 "k8s.io/api/core/v1"
)

func TestAggregateErrorCounters(t *testing.T) {
	counters := func(kv map[string]int) *archive.ErrorCounter {
		ec := archive.ErrorCounter(kv)
		return &ec
	}

	t.Run("upgrade mode sums the upgrade plugin only", func(t *testing.T) {
		reResult := &ReportResult{Plugins: map[string]*ReportPlugin{
			plugin.PluginNameOpenShiftUpgrade: {
				ErrorCounters: counters(map[string]int{"Failed": 3, "total": 3}),
			},
			plugin.PluginNameKubernetesConformance: {
				ErrorCounters: counters(map[string]int{"Failed": 90, "total": 90}),
			},
		}}
		got := aggregateErrorCounters(reResult, plugin.WorkflowUpgrade)
		assert.Equal(t, 3, (*got)["total"],
			"upgrade mode must ignore the conformance plugins")
	})

	t.Run("upgrade mode reports a measured zero when no pattern matched", func(t *testing.T) {
		reResult := &ReportResult{Plugins: map[string]*ReportPlugin{
			plugin.PluginNameOpenShiftUpgrade: {
				ErrorCounters: counters(map[string]int{}),
			},
		}}
		got := aggregateErrorCounters(reResult, plugin.WorkflowUpgrade)
		total, ok := (*got)["total"]
		assert.True(t, ok, "a processed plugin must report an explicit total")
		assert.Equal(t, 0, total)
	})

	t.Run("upgrade mode leaves the total absent when the plugin is missing", func(t *testing.T) {
		reResult := &ReportResult{Plugins: map[string]*ReportPlugin{}}
		got := aggregateErrorCounters(reResult, plugin.WorkflowUpgrade)
		_, ok := (*got)["total"]
		assert.False(t, ok, "an unprocessed plugin must not be reported as zero errors")
	})

	t.Run("regular mode sums both conformance plugins", func(t *testing.T) {
		reResult := &ReportResult{Plugins: map[string]*ReportPlugin{
			plugin.PluginNameKubernetesConformance: {
				ErrorCounters: counters(map[string]int{"Failed": 10, "total": 10}),
			},
			plugin.PluginNameOpenShiftConformance: {
				ErrorCounters: counters(map[string]int{"Failed": 5, "timed out": 2, "total": 7}),
			},
			plugin.PluginNameOpenShiftUpgrade: {
				ErrorCounters: counters(map[string]int{"Failed": 99, "total": 99}),
			},
		}}
		got := aggregateErrorCounters(reResult, "regular")
		assert.Equal(t, 17, (*got)["total"], "regular mode must ignore the upgrade plugin")
		assert.Equal(t, 15, (*got)["Failed"], "per-pattern counters must accumulate")
		assert.Equal(t, 2, (*got)["timed out"])
	})

	t.Run("a processed plugin without counters still reports zero", func(t *testing.T) {
		reResult := &ReportResult{Plugins: map[string]*ReportPlugin{
			plugin.PluginNameKubernetesConformance: {ErrorCounters: nil},
		}}
		got := aggregateErrorCounters(reResult, "regular")
		total, ok := (*got)["total"]
		assert.True(t, ok)
		assert.Equal(t, 0, total)
	})
}

const upgradePluginID = "05-openshift-cluster-upgrade"

func TestPopulateUpgradePluginConformance(t *testing.T) {
	const failedTest = "[sig-arch] Cluster should remain functional during upgrade"
	result := summary.NewConsolidatedSummary(&summary.ConsolidatedSummaryInput{}).Provider
	result.OpenShift.PluginResultConformanceUpgrade = &plugin.OPCTPluginSummary{
		Name:           plugin.PluginNameOpenShiftUpgrade,
		Status:         "failed",
		Total:          2,
		Passed:         1,
		Failed:         1,
		FailedFilter1:  []string{failedTest},
		FailedFilter2:  []string{failedTest},
		FailedFilter3:  []string{failedTest},
		FailedFilter4:  []string{failedTest},
		FailedFilter5:  []string{failedTest},
		FailedFilter6:  []string{failedTest},
		FailedFiltered: []string{failedTest},
		Tests: plugin.Tests{
			failedTest: {
				ID:            upgradePluginID + "-0",
				Name:          failedTest,
				Status:        "failed",
				ErrorCounters: archive.ErrorCounter{"total": 1},
			},
		},
	}
	result.Suites.UpgradeConformance.Tests = []string{failedTest}
	result.Suites.UpgradeConformance.Count = 1
	result.Sonobuoy.SetPluginDefinition(upgradePluginID, &summary.SonobuoyPluginDefinition{
		Definition: &manifest.Manifest{
			SonobuoyConfig: manifest.SonobuoyConfig{PluginName: upgradePluginID},
			Spec:           manifest.Container{Container: v1.Container{Image: "quay.io/opct/plugin:test"}},
		},
		SonobuoyImage: "quay.io/opct/sonobuoy:test",
	})

	reportData := NewReportData(false)
	reportData.Summary = &ReportSummary{
		Alerts:  &ReportSummaryAlerts{},
		Runtime: &ReportSummaryRuntime{},
	}
	reportResult := &ReportResult{Plugins: map[string]*ReportPlugin{}}

	require.NoError(t, reportData.populatePluginConformance(result, reportResult, upgradePluginID))
	upgrade, ok := reportResult.Plugins[upgradePluginID]
	require.True(t, ok)
	assert.Equal(t, "Results for OpenShift Conformance Upgrade Suite", upgrade.Title)
	assert.Equal(t, int64(1), upgrade.Stat.FilterSuite)
	assert.Equal(t, int64(1), upgrade.Stat.FilterFailures)
	assert.Equal(t, result.Suites.UpgradeConformance, upgrade.Suite)
	assert.Len(t, upgrade.FailedFiltered, 1)
	assert.Equal(t, failedTest, upgrade.FailedFiltered[0].Name)
	assert.Equal(t, 1, (*upgrade.ErrorCounters)["total"])
	require.NotNil(t, upgrade.Definition)
	assert.Equal(t, upgradePluginID, upgrade.Definition.Name)
	assert.Equal(t, "quay.io/opct/plugin:test", upgrade.Definition.PluginImage)
}

func TestReportTemplateSuiteUpgradeVisibility(t *testing.T) {
	t.Run("upgrade result is available", func(t *testing.T) {
		rendered := renderReportTemplate(t, plugin.WorkflowUpgrade, true)
		assert.Contains(t, rendered,
			`v-on:click="changeMenu('05-openshift-cluster-upgrade')"`)
		assert.Contains(t, rendered, "Suite Upgrade")
	})

	t.Run("regular archive is not applicable", func(t *testing.T) {
		rendered := renderReportTemplate(t, "", false)
		assert.NotContains(t, rendered,
			`v-on:click="changeMenu('05-openshift-cluster-upgrade')"`)
		assert.NotContains(t, rendered, ">Suite Upgrade<")
	})

	t.Run("regular archive with a skipped plugin is not applicable", func(t *testing.T) {
		rendered := renderReportTemplate(t, "", true)
		assert.NotContains(t, rendered,
			`v-on:click="changeMenu('05-openshift-cluster-upgrade')"`)
		assert.NotContains(t, rendered, ">Suite Upgrade<")
	})
}

func TestSaveResultsRendersSuiteUpgrade(t *testing.T) {
	originalFS := efs.GetData()
	efs.UpdateData(os.DirFS("../.."))
	t.Cleanup(func() {
		efs.UpdateData(originalFS)
	})

	reportData := NewReportData(false)
	reportData.Summary = &ReportSummary{
		Alerts:  &ReportSummaryAlerts{},
		Runtime: &ReportSummaryRuntime{Timers: metrics.NewTimers()},
	}
	reportData.Provider = &ReportResult{IsUpgradeWorkflow: true, Plugins: map[string]*ReportPlugin{
		upgradePluginID: {Name: upgradePluginID, Tests: map[string]*plugin.TestItem{}},
	}}
	reportData.Setup.API.Workflow = plugin.WorkflowUpgrade

	outputDir := t.TempDir()
	require.NoError(t, reportData.SaveResults(outputDir))
	index, err := os.ReadFile(outputDir + "/index.html")
	require.NoError(t, err)
	assert.Contains(t, string(index), "Suite Upgrade")
	assert.Contains(t, string(index),
		`v-on:click="changeMenu('05-openshift-cluster-upgrade')"`)
	require.FileExists(t, outputDir+"/opct-report.json")
}

func renderReportTemplate(t *testing.T, workflow string, includeUpgrade bool) string {
	t.Helper()
	templateData, err := os.ReadFile("../../data/templates/report/report.html")
	require.NoError(t, err)

	reportData := NewReportData(false)
	reportData.Summary = &ReportSummary{Alerts: &ReportSummaryAlerts{}}
	reportData.Provider = &ReportResult{Plugins: map[string]*ReportPlugin{}}
	reportData.Provider.IsUpgradeWorkflow = workflow == plugin.WorkflowUpgrade
	if includeUpgrade {
		reportData.Provider.Plugins[upgradePluginID] = &ReportPlugin{Name: upgradePluginID}
	}

	tmpl, err := template.New("report").Delims("[[", "]]").Parse(string(templateData))
	require.NoError(t, err)
	var rendered bytes.Buffer
	require.NoError(t, tmpl.Execute(&rendered, reportData))
	return strings.TrimSpace(rendered.String())
}
