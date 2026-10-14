package report

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/archive"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/metrics"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/summary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmware-tanzu/sonobuoy/pkg/discovery"
)

// OPCT-434: fixture-driven integration test exercising the real
// ReportData.Populate() entrypoint (the same path "opct report" uses) for
// each streamlined workflow mode, without requiring a real archive. It
// asserts the final report aggregates exactly the plugins applicable to the
// workflow, with no empty sections, errors, or misleading entries for
// plugins the workflow intentionally does not run.

// newWorkflowFixture builds a minimal, valid ConsolidatedSummary for a given
// workflow ("regular" or "upgrade"), with just enough cluster metadata set to
// avoid the pre-existing nil-safety requirements of ReportData.Populate (e.g.
// infra.Status.PlatformStatus, Sonobuoy cluster health). Only the plugins
// relevant to the workflow are populated, mirroring what a real archive
// collected under a streamlined workflow would contain.
func newWorkflowFixture(t *testing.T, workflow string) *summary.ConsolidatedSummary {
	t.Helper()

	cs := summary.NewConsolidatedSummary(&summary.ConsolidatedSummaryInput{
		Archive: "fixture-provider.tar.gz",
		Timers:  metrics.NewTimers(),
	})

	cs.Provider.OpenShift.Infrastructure = &configv1.Infrastructure{
		Status: configv1.InfrastructureStatus{
			PlatformStatus: &configv1.PlatformStatus{Type: configv1.AWSPlatformType},
		},
	}
	cs.Provider.OpenShift.ClusterVersion = &configv1.ClusterVersion{
		Status: configv1.ClusterVersionStatus{Desired: configv1.Release{Version: "4.20.1"}},
	}
	cs.Provider.OpenShift.ClusterOperators = &configv1.ClusterOperatorList{}
	cs.Provider.OpenShift.ClusterNetwork = &configv1.Network{}
	cs.Provider.Sonobuoy.Cluster = &discovery.ClusterSummary{
		NodeHealth: discovery.HealthInfo{Total: 1, Healthy: 1},
		PodHealth:  discovery.HealthInfo{Total: 1, Healthy: 1},
	}
	cs.Provider.Sonobuoy.OpctConfig = []*archive.RuntimeInfoItem{
		{Name: "run-mode", Value: workflow},
	}

	switch workflow {
	case plugin.WorkflowUpgrade:
		cs.Provider.OpenShift.PluginResultConformanceUpgrade = &plugin.OPCTPluginSummary{
			Name: plugin.PluginNameOpenShiftUpgrade, Status: "passed",
			Total: 40, Passed: 30, Skipped: 10,
		}
		cs.Provider.OpenShift.PluginResultArtifactsCollector = &plugin.OPCTPluginSummary{
			Name: plugin.PluginNameArtifactsCollector, Status: "passed",
			Total: 13, Passed: 13,
		}
	default: // regular / disconnected
		cs.Provider.OpenShift.PluginResultK8sConformance = &plugin.OPCTPluginSummary{
			Name: plugin.PluginNameKubernetesConformance, Status: "passed",
			Total: 10, Passed: 10,
		}
		cs.Provider.OpenShift.PluginResultOCPValidated = &plugin.OPCTPluginSummary{
			Name: plugin.PluginNameOpenShiftConformance, Status: "passed",
			Total: 10, Passed: 10,
		}
		cs.Provider.OpenShift.PluginResultArtifactsCollector = &plugin.OPCTPluginSummary{
			Name: plugin.PluginNameArtifactsCollector, Status: "passed",
			Total: 13, Passed: 13,
		}
	}

	return cs
}

func TestPopulateReportPerWorkflowMode(t *testing.T) {
	tests := []struct {
		name            string
		workflow        string
		expectedPlugins []string
		excludedPlugins []string
	}{
		{
			name:            "conformance-upgrade: aggregates 05 and artifacts, no reference to 10/20",
			workflow:        plugin.WorkflowUpgrade,
			expectedPlugins: []string{plugin.PluginNameOpenShiftUpgrade, plugin.PluginNameArtifactsCollector},
			excludedPlugins: []string{plugin.PluginNameKubernetesConformance, plugin.PluginNameOpenShiftConformance, plugin.PluginNameConformanceReplay},
		},
		{
			name: "conformance: aggregates 10/20 and artifacts, no reference to 05",
			// "regular" is not yet an exported plugin.Workflow* constant on
			// this base branch; newWorkflowFixture treats any non-upgrade
			// value the same via its default case.
			workflow:        "regular",
			expectedPlugins: []string{plugin.PluginNameKubernetesConformance, plugin.PluginNameOpenShiftConformance, plugin.PluginNameArtifactsCollector},
			excludedPlugins: []string{plugin.PluginNameOpenShiftUpgrade, plugin.PluginNameConformanceReplay},
		},
		{
			// conformance-disconnected must preserve the same plugin set as
			// conformance (05 excluded, 10/20 present).
			name:            "conformance-disconnected: preserves existing (non-upgrade) behavior",
			workflow:        "",
			expectedPlugins: []string{plugin.PluginNameKubernetesConformance, plugin.PluginNameOpenShiftConformance, plugin.PluginNameArtifactsCollector},
			excludedPlugins: []string{plugin.PluginNameOpenShiftUpgrade, plugin.PluginNameConformanceReplay},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := newWorkflowFixture(t, tt.workflow)

			re := NewReportData(false)
			var popErr error
			require.NotPanics(t, func() {
				popErr = re.Populate(cs)
			}, "report generation must not panic for workflow %q", tt.workflow)
			require.NoError(t, popErr)

			for _, name := range tt.expectedPlugins {
				p, ok := re.Provider.Plugins[name]
				assert.True(t, ok, "expected plugin %s to be present in the report", name)
				if ok {
					assert.NotNil(t, p.Stat, "plugin %s must have aggregated stats, not an empty section", name)
				}
			}
			for _, name := range tt.excludedPlugins {
				_, ok := re.Provider.Plugins[name]
				assert.False(t, ok, "plugin %s is intentionally not run by this workflow and must not appear in the report", name)
			}
		})
	}
}

// TestPopulateReportUpgradeAggregationAccuracy locks in the exact counters
// observed in the real CI artifact (job 2109072457313816576) that originally
// surfaced the "unable to get plugin" symptom, so a regression in the
// aggregation path (not just the CLI rendering) is caught.
func TestPopulateReportUpgradeAggregationAccuracy(t *testing.T) {
	cs := newWorkflowFixture(t, plugin.WorkflowUpgrade)

	re := NewReportData(false)
	require.NoError(t, re.Populate(cs))

	upgrade, ok := re.Provider.Plugins[plugin.PluginNameOpenShiftUpgrade]
	require.True(t, ok)
	assert.Equal(t, int64(40), upgrade.Stat.Total)
	assert.Equal(t, int64(30), upgrade.Stat.Passed)
	assert.Equal(t, int64(0), upgrade.Stat.Failed)
	assert.Equal(t, int64(10), upgrade.Stat.Skipped)

	collector, ok := re.Provider.Plugins[plugin.PluginNameArtifactsCollector]
	require.True(t, ok)
	assert.Equal(t, int64(13), collector.Stat.Passed)
}
