package summary

import (
	"os"
	"path/filepath"
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

func TestSaveUpgradeFailureDetails(t *testing.T) {
	const failedUpgradeTest = "[sig-arch] Cluster should remain functional during upgrade"
	const failure = "upgrade failure output"
	const systemOut = "upgrade system output"

	cs := NewConsolidatedSummary(&ConsolidatedSummaryInput{})
	cs.Verbose = true
	cs.Provider.OpenShift.PluginResultConformanceUpgrade = &plugin.OPCTPluginSummary{
		Name:       plugin.PluginNameOpenShiftUpgrade,
		FailedList: []string{failedUpgradeTest},
		Tests: plugin.Tests{
			failedUpgradeTest: {
				ID:        "05-openshift-cluster-upgrade-0",
				Name:      failedUpgradeTest,
				Failure:   failure,
				SystemOut: systemOut,
			},
		},
	}
	cs.Provider.Suites.UpgradeConformance.Tests = []string{failedUpgradeTest}

	outputDir := t.TempDir()
	require.NoError(t, cs.saveResultsPlugin(outputDir, plugin.PluginNameOpenShiftUpgrade))
	require.FileExists(t, filepath.Join(outputDir,
		"tests_05-openshift-cluster-upgrade_suite_full.txt"))

	require.NoError(t, cs.extractFailuresDetailsByPlugin(outputDir, plugin.PluginNameOpenShiftUpgrade))
	failurePath := filepath.Join(outputDir, "failures-05-openshift-cluster-upgrade",
		"05-openshift-cluster-upgrade-0-failure.txt")
	systemOutPath := filepath.Join(outputDir, "failures-05-openshift-cluster-upgrade",
		"05-openshift-cluster-upgrade-0-systemOut.txt")

	gotFailure, err := os.ReadFile(failurePath)
	require.NoError(t, err)
	assert.Equal(t, failure, string(gotFailure))
	gotSystemOut, err := os.ReadFile(systemOutPath)
	require.NoError(t, err)
	assert.Equal(t, systemOut, string(gotSystemOut))
}
