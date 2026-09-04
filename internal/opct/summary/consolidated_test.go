package summary

import (
	"testing"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyFilterSuiteUpgradeOnlyArchive(t *testing.T) {
	const failedUpgradeTest = "[sig-arch] Cluster should remain functional during upgrade"

	// Upgrade archives contain the upgrade and collector plugins, but omit the
	// conformance and replay plugins. This models the minimal state parsed from
	// an upgrade-only Sonobuoy archive.
	cs := NewConsolidatedSummary(&ConsolidatedSummaryInput{})
	upgradePlugin := &plugin.OPCTPluginSummary{
		Name:       plugin.PluginNameOpenShiftUpgrade,
		FailedList: []string{failedUpgradeTest},
		Tests: plugin.Tests{
			failedUpgradeTest: {
				Name: failedUpgradeTest,
			},
		},
	}
	cs.Provider.OpenShift.PluginResultConformanceUpgrade = upgradePlugin
	cs.Provider.OpenShift.PluginResultArtifactsCollector = &plugin.OPCTPluginSummary{
		Name: plugin.PluginNameArtifactsCollector,
	}

	require.NotPanics(t, func() {
		require.NoError(t, cs.applyFilterSuite())
	})

	assert.Equal(t, []string{failedUpgradeTest}, upgradePlugin.FailedFilter1)
	assert.Equal(t, "filter1SuiteOnly", upgradePlugin.Tests[failedUpgradeTest].State)
}
