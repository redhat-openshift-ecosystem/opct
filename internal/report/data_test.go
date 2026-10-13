package report

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/archive"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
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
