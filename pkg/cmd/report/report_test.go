package report

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/summary"
	"github.com/redhat-openshift-ecosystem/opct/internal/report"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it. The table writer used by showErrorDetailPlugin
// mirrors output directly to the process's os.Stdout, so log-only capture
// (as logrus allows) is not enough to assert on rendered sections.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

// OPCT-434: report aggregation must not treat a plugin intentionally omitted
// by the workflow (10/20 in upgrade mode, 05 in regular/disconnected mode) as
// a reporting error or an empty/misleading section.

func newPlugin(id, status string, total, passed, failed int64) *report.ReportPlugin {
	return &report.ReportPlugin{
		ID:   id,
		Name: id,
		Stat: &report.ReportPluginStat{
			Status: status,
			Total:  total,
			Passed: passed,
			Failed: failed,
		},
	}
}

func TestPresentConformanceDetailPlugins(t *testing.T) {
	t.Run("upgrade mode: only 05 present, 10/20 absent", func(t *testing.T) {
		re := &report.ReportResult{Plugins: map[string]*report.ReportPlugin{
			plugin.PluginNameOpenShiftUpgrade:   newPlugin(plugin.PluginNameOpenShiftUpgrade, "passed", 40, 30, 0),
			plugin.PluginNameArtifactsCollector: newPlugin(plugin.PluginNameArtifactsCollector, "passed", 13, 13, 0),
		}}
		got := presentConformanceDetailPlugins(re)
		assert.Len(t, got, 1, "only the upgrade plugin should be present")
		assert.Equal(t, plugin.PluginNameOpenShiftUpgrade, got[0].ID)
	})

	t.Run("regular mode: 10/20 present, 05 absent", func(t *testing.T) {
		re := &report.ReportResult{Plugins: map[string]*report.ReportPlugin{
			plugin.PluginNameKubernetesConformance: newPlugin(plugin.PluginNameKubernetesConformance, "passed", 10, 10, 0),
			plugin.PluginNameOpenShiftConformance:  newPlugin(plugin.PluginNameOpenShiftConformance, "passed", 10, 10, 0),
		}}
		got := presentConformanceDetailPlugins(re)
		assert.Len(t, got, 2)
		assert.Equal(t, plugin.PluginNameKubernetesConformance, got[0].ID)
		assert.Equal(t, plugin.PluginNameOpenShiftConformance, got[1].ID)
	})

	t.Run("no conformance plugins present at all", func(t *testing.T) {
		re := &report.ReportResult{Plugins: map[string]*report.ReportPlugin{}}
		got := presentConformanceDetailPlugins(re)
		assert.Len(t, got, 0)
	})
}

func TestShowErrorDetailsNoErrorForAbsentPlugins(t *testing.T) {
	var buf bytes.Buffer
	orig := log.StandardLogger().Out
	log.SetOutput(&buf)
	log.SetLevel(log.DebugLevel)
	defer func() {
		log.SetOutput(orig)
		log.SetLevel(log.InfoLevel)
	}()

	// Simulates the upgrade-mode archive from job 2109072457313816576:
	// plugins 10/20 are absent by workflow design, not a failure.
	re := &report.ReportData{
		Provider: &report.ReportResult{Plugins: map[string]*report.ReportPlugin{
			plugin.PluginNameOpenShiftUpgrade:   newPlugin(plugin.PluginNameOpenShiftUpgrade, "passed", 40, 30, 0),
			plugin.PluginNameArtifactsCollector: newPlugin(plugin.PluginNameArtifactsCollector, "passed", 13, 13, 0),
		}},
	}

	var err error
	stdout := captureStdout(t, func() {
		err = showErrorDetails(re, false)
	})
	assert.NoError(t, err)
	assert.NotContains(t, buf.String(), "unable to get plugin",
		"absent plugins 10/20 in upgrade mode must not be logged as errors")
	assert.Empty(t, stdout,
		"a passing upgrade plugin has nothing to review and must not print an empty details section")
}

func TestShowErrorDetailPluginNilIsNoop(t *testing.T) {
	assert.NotPanics(t, func() {
		showErrorDetailPlugin(nil, false)
	})
}

func TestHasRenderableDetail(t *testing.T) {
	t.Run("nil plugin", func(t *testing.T) {
		assert.False(t, hasRenderableDetail(nil, false))
	})

	t.Run("passing upgrade plugin (no raw failures)", func(t *testing.T) {
		p := newPlugin(plugin.PluginNameOpenShiftUpgrade, "passed", 40, 30, 0)
		assert.False(t, hasRenderableDetail(p, false))
	})

	t.Run("failing upgrade plugin has raw failures", func(t *testing.T) {
		p := newPlugin(plugin.PluginNameOpenShiftUpgrade, "failed", 40, 38, 2)
		p.Tests = map[string]*plugin.TestItem{
			"ok":   {Name: "ok", Status: "passed"},
			"fail": {Name: "fail", Status: "failed"},
		}
		assert.True(t, hasRenderableDetail(p, false))
	})

	t.Run("passing conformance plugin (no filtered failures)", func(t *testing.T) {
		p := newPlugin(plugin.PluginNameKubernetesConformance, "passed", 10, 10, 0)
		assert.False(t, hasRenderableDetail(p, false))
	})

	t.Run("failing conformance plugin has filtered failures", func(t *testing.T) {
		p := newPlugin(plugin.PluginNameKubernetesConformance, "failed", 10, 9, 1)
		p.FailedFiltered = []*report.ReportTestFailure{{Name: "some-test"}}
		assert.True(t, hasRenderableDetail(p, false))
	})

	t.Run("verbose-only filter is not renderable without verbose", func(t *testing.T) {
		p := newPlugin(plugin.PluginNameKubernetesConformance, "passed", 10, 10, 0)
		p.FailedFilter1 = []*report.ReportTestFailure{{Name: "excluded-by-suite"}}
		assert.False(t, hasRenderableDetail(p, false))
		assert.True(t, hasRenderableDetail(p, true))
	})
}

// TestShowErrorDetailsNoEmptySectionForPassingRun is the regression test for
// the empty-header issue: a passing plugin (nothing to review) must not
// produce an empty "test failures" header, which would read as a misleading
// status entry for a plugin that actually ran and passed.
func TestShowErrorDetailsNoEmptySectionForPassingRun(t *testing.T) {
	re := &report.ReportData{
		Provider: &report.ReportResult{Plugins: map[string]*report.ReportPlugin{
			plugin.PluginNameOpenShiftUpgrade:   newPlugin(plugin.PluginNameOpenShiftUpgrade, "passed", 40, 30, 0),
			plugin.PluginNameArtifactsCollector: newPlugin(plugin.PluginNameArtifactsCollector, "passed", 13, 13, 0),
		}},
	}

	stdout := captureStdout(t, func() {
		require.NoError(t, showErrorDetails(re, false))
	})
	assert.Empty(t, stdout, "no plugin has failures to review, so no section should print at all")
}

func TestShowErrorDetailsRendersFailingUpgradePlugin(t *testing.T) {
	upgrade := newPlugin(plugin.PluginNameOpenShiftUpgrade, "failed", 40, 38, 2)
	upgrade.Tests = map[string]*plugin.TestItem{
		"test-pass": {Name: "test-pass", Status: "passed"},
		"test-fail": {Name: "test-fail", Status: "failed"},
	}
	re := &report.ReportData{
		Provider: &report.ReportResult{Plugins: map[string]*report.ReportPlugin{
			plugin.PluginNameOpenShiftUpgrade: upgrade,
		}},
	}

	stdout := captureStdout(t, func() {
		require.NoError(t, showErrorDetails(re, false))
	})
	assert.Contains(t, stdout, "Result details by conformance plugins")
	assert.Contains(t, stdout, "test-fail")
	assert.NotContains(t, stdout, "test-pass")
}

func TestCollectRawFailedTests(t *testing.T) {
	tests := map[string]*plugin.TestItem{
		"test-a": {Name: "test-a", Status: "failed"},
		"test-b": {Name: "test-b", Status: "passed"},
		"test-c": {Name: "test-c", Status: "failed"},
		"test-d": {Name: "test-d", Status: "skipped"},
		"test-e": nil,
	}
	got := collectRawFailedTests(tests)
	assert.Equal(t, []string{"test-a", "test-c"}, got)
}

func TestCollectRawFailedTestsEmpty(t *testing.T) {
	assert.Equal(t, []string{}, collectRawFailedTests(nil))
	assert.Equal(t, []string{}, collectRawFailedTests(map[string]*plugin.TestItem{}))
}

// TestPluginSummaryRows covers the OPCT-434 crash fix: provider and baseline
// archives can come from different workflows, so a plugin present in the
// provider is not guaranteed to exist in the baseline. Looking it up without
// a presence check previously caused a nil pointer dereference.
func TestPluginSummaryRows(t *testing.T) {
	provider := map[string]*report.ReportPlugin{
		plugin.PluginNameOpenShiftUpgrade: newPlugin(plugin.PluginNameOpenShiftUpgrade, "passed", 40, 30, 0),
	}

	t.Run("plugin absent from provider", func(t *testing.T) {
		provRow, basRow, ok := pluginSummaryRows(provider, nil, plugin.PluginNameKubernetesConformance, false)
		assert.False(t, ok)
		assert.Nil(t, provRow)
		assert.Nil(t, basRow)
	})

	t.Run("no baseline processed", func(t *testing.T) {
		provRow, basRow, ok := pluginSummaryRows(provider, nil, plugin.PluginNameOpenShiftUpgrade, false)
		assert.True(t, ok)
		assert.NotNil(t, provRow)
		assert.Nil(t, basRow)
	})

	t.Run("baseline processed but plugin absent from baseline (workflow mismatch)", func(t *testing.T) {
		baseline := map[string]*report.ReportPlugin{
			plugin.PluginNameKubernetesConformance: newPlugin(plugin.PluginNameKubernetesConformance, "passed", 10, 10, 0),
		}
		assert.NotPanics(t, func() {
			provRow, basRow, ok := pluginSummaryRows(provider, baseline, plugin.PluginNameOpenShiftUpgrade, true)
			assert.True(t, ok, "provider row must still be produced")
			assert.NotNil(t, provRow)
			// When baselineProcessed is true, only rowsPBas is rendered by the
			// caller (rowsProv is built but never shown) — so basRow must still
			// be produced, with an empty baseline cell, or the provider's
			// plugin result silently disappears from the actual output.
			require.NotNil(t, basRow, "baseline row must still be produced so the provider result is not dropped from the rendered table")
			assert.Equal(t, "", basRow[2], "baseline cell must be empty, not fabricated")
		})
	})

	t.Run("baseline processed and plugin present in both", func(t *testing.T) {
		baseline := map[string]*report.ReportPlugin{
			plugin.PluginNameOpenShiftUpgrade: newPlugin(plugin.PluginNameOpenShiftUpgrade, "passed", 40, 28, 2),
		}
		provRow, basRow, ok := pluginSummaryRows(provider, baseline, plugin.PluginNameOpenShiftUpgrade, true)
		assert.True(t, ok)
		assert.NotNil(t, provRow)
		assert.NotNil(t, basRow)
	})

	t.Run("baseline processed but plugin absent from provider (workflow mismatch)", func(t *testing.T) {
		baseline := map[string]*report.ReportPlugin{
			plugin.PluginNameKubernetesConformance: newPlugin(plugin.PluginNameKubernetesConformance, "passed", 10, 10, 0),
		}
		provRow, basRow, ok := pluginSummaryRows(provider, baseline, plugin.PluginNameKubernetesConformance, true)
		assert.True(t, ok, "baseline row must still be produced")
		assert.Nil(t, provRow, "no provider cell to show")
		require.NotNil(t, basRow, "baseline's real result must not be dropped just because the provider workflow omits this plugin")
		assert.Equal(t, "", basRow[1], "provider cell must be empty, not fabricated")
		assert.Contains(t, basRow[2], "10/10", "baseline's actual result must be shown")
	})
}

// newRenderableReport builds the minimum ReportData that
// showReportAggregatedSummary dereferences, so the summary table can be
// rendered end to end in a test.
func newRenderableResult(plugins map[string]*report.ReportPlugin) *report.ReportResult {
	return &report.ReportResult{
		Version: &report.ReportVersion{
			OpenShift:  &summary.SummaryClusterVersionOutput{Desired: "4.20.1"},
			Kubernetes: "v1.33.1",
		},
		Infra:            &report.ReportInfra{Name: "test-infra", PlatformType: "AWS"},
		ClusterOperators: &report.ReportClusterOperators{CountAvailable: 30},
		ClusterHealth:    &report.ReportClusterHealth{NodeHealthTotal: 1, NodeHealthy: 1, PodHealthTotal: 1, PodHealthy: 1},
		Plugins:          plugins,
	}
}

// TestShowReportAggregatedSummaryBaselineMismatchKeepsProviderRow is the
// rendered-output regression test for the baseline-mismatch path. The
// helper-level test alone could not catch it: when a baseline is processed
// the CLI renders only the provider+baseline table, so returning no baseline
// row dropped the provider's plugin result from the visible report entirely.
func TestShowReportAggregatedSummaryBaselineMismatchKeepsProviderRow(t *testing.T) {
	// Provider ran upgrade (05 present); baseline came from a regular run
	// (10 present, 05 absent) — a provider/baseline workflow mismatch.
	re := &report.ReportData{
		Summary: &report.ReportSummary{
			Tests: &report.ReportSummaryTests{Archive: "provider.tar.gz", ArchiveDiff: "baseline.tar.gz"},
		},
		Provider: newRenderableResult(map[string]*report.ReportPlugin{
			plugin.PluginNameOpenShiftUpgrade: newPlugin(plugin.PluginNameOpenShiftUpgrade, "passed", 40, 30, 0),
		}),
		Baseline: newRenderableResult(map[string]*report.ReportPlugin{
			plugin.PluginNameKubernetesConformance: newPlugin(plugin.PluginNameKubernetesConformance, "passed", 10, 10, 0),
		}),
	}

	stdout := captureStdout(t, func() {
		require.NoError(t, showReportAggregatedSummary(re))
	})

	assert.Contains(t, stdout, plugin.PluginNameOpenShiftUpgrade,
		"the provider's plugin must stay visible in the rendered table even though the baseline lacks it")
	assert.Contains(t, stdout, "passed [40/30/0/0]",
		"the provider's counters must be rendered, not dropped or fabricated")
}
