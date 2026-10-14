package report

import (
	"bytes"
	"testing"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/redhat-openshift-ecosystem/opct/internal/report"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

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

	err := showErrorDetails(re, false)
	assert.NoError(t, err)
	assert.NotContains(t, buf.String(), "unable to get plugin",
		"absent plugins 10/20 in upgrade mode must not be logged as errors")
}

func TestShowErrorDetailPluginNilIsNoop(t *testing.T) {
	assert.NotPanics(t, func() {
		showErrorDetailPlugin(nil, false)
	})
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
			assert.Nil(t, basRow, "baseline row must be omitted, not fabricated")
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
}
