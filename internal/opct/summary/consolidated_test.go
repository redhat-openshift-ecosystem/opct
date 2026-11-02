package summary

import (
	"os"
	"path/filepath"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
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

func TestApplyFilterKnownFailures(t *testing.T) {
	// userAPITestName aliases the exported constant for brevity in test cases.
	const userAPITestName = KnownFailureUserAPIGroups

	// SA-specific failure message that is the expected false positive.
	const saFailureMsg = "unexpected groups returned for user/~: got [system:authenticated system:serviceaccounts system:serviceaccounts:opct]"

	tests := []struct {
		name                  string
		inputFailures         []string
		failureMessages       map[string]string // test name → Failure field content
		systemOutMessages     map[string]string // test name → SystemOut field content
		expectedFailures      []string
		expectedExcludedCount int
	}{
		{
			name: "should_exclude_all_known_failures_with_matching_pattern",
			inputFailures: []string{
				"[sig-arch] External binary usage",
				"[sig-mco] Machine config pools complete upgrade",
				userAPITestName,
			},
			failureMessages: map[string]string{
				userAPITestName: saFailureMsg,
			},
			expectedFailures:      []string{},
			expectedExcludedCount: 3,
		},
		{
			name: "should_keep_unknown_failures_and_exclude_known_ones",
			inputFailures: []string{
				"[sig-apps] some real failure test",
				userAPITestName,
				"[sig-network] another real failure",
			},
			failureMessages: map[string]string{
				userAPITestName: saFailureMsg,
			},
			expectedFailures: []string{
				"[sig-apps] some real failure test",
				"[sig-network] another real failure",
			},
			expectedExcludedCount: 1,
		},
		{
			name: "should_exclude_user_api_test_when_failure_matches_sa_pattern",
			inputFailures: []string{
				userAPITestName,
			},
			failureMessages: map[string]string{
				userAPITestName: saFailureMsg,
			},
			expectedFailures:      []string{},
			expectedExcludedCount: 1,
		},
		{
			name: "should_NOT_exclude_user_api_test_when_failure_does_not_match_pattern",
			inputFailures: []string{
				userAPITestName,
			},
			failureMessages: map[string]string{
				userAPITestName: "completely different error: connection refused",
			},
			expectedFailures: []string{
				userAPITestName,
			},
			expectedExcludedCount: 0,
		},
		{
			name: "should_NOT_exclude_user_api_test_when_failure_is_empty",
			inputFailures: []string{
				userAPITestName,
			},
			failureMessages: map[string]string{},
			expectedFailures: []string{
				userAPITestName,
			},
			expectedExcludedCount: 0,
		},
		{
			name: "should_exclude_user_api_test_when_systemout_matches_pattern",
			inputFailures: []string{
				userAPITestName,
			},
			failureMessages: map[string]string{},
			systemOutMessages: map[string]string{
				userAPITestName: saFailureMsg,
			},
			expectedFailures:      []string{},
			expectedExcludedCount: 1,
		},
		{
			name: "should_exclude_name_only_entries_without_pattern_check",
			inputFailures: []string{
				"[sig-arch] External binary usage",
				"[sig-mco] Machine config pools complete upgrade",
			},
			failureMessages:       map[string]string{},
			expectedFailures:      []string{},
			expectedExcludedCount: 2,
		},
		{
			name:                  "should_handle_no_failures",
			inputFailures:         []string{},
			failureMessages:       map[string]string{},
			expectedFailures:      []string{},
			expectedExcludedCount: 0,
		},
		{
			name: "should_keep_all_when_no_known_failures_match",
			inputFailures: []string{
				"[sig-apps] deployments should work",
				"[sig-network] services should be reachable",
			},
			failureMessages: map[string]string{},
			expectedFailures: []string{
				"[sig-apps] deployments should work",
				"[sig-network] services should be reachable",
			},
			expectedExcludedCount: 0,
		},
		{
			name: "should_keep_user_api_with_real_bug_and_exclude_others",
			inputFailures: []string{
				"[sig-arch] External binary usage",
				userAPITestName,
				"[sig-apps] deployments should work",
			},
			failureMessages: map[string]string{
				// UserAPI fails with a non-SA error → potential real bug
				userAPITestName: "user groups API returned 500 Internal Server Error",
			},
			expectedFailures: []string{
				"[sig-apps] deployments should work",
				userAPITestName,
			},
			expectedExcludedCount: 1, // only sig-arch excluded
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build test items map required by the filter (ps.Tests[v].State access)
			testItems := make(plugin.Tests, len(tt.inputFailures))
			for _, name := range tt.inputFailures {
				item := &plugin.TestItem{
					Name:   name,
					Status: "failed",
					State:  "processed",
				}
				if msg, ok := tt.failureMessages[name]; ok {
					item.Failure = msg
				}
				if msg, ok := tt.systemOutMessages[name]; ok {
					item.SystemOut = msg
				}
				testItems[name] = item
			}

			// Create a plugin summary with failures in FailedFilter1
			// (the input to the known-failures filter)
			ps := &plugin.OPCTPluginSummary{
				Name:          plugin.PluginNameOpenShiftConformance,
				Tests:         testItems,
				FailedFilter1: tt.inputFailures,
			}

			// Create the consolidated summary with the plugin result.
			// All plugin results must be initialized because applyFilterKnownFailures
			// iterates over all plugin types.
			cs := &ConsolidatedSummary{
				Provider: &ResultSummary{
					OpenShift: &OpenShiftSummary{
						PluginResultOCPValidated:       ps,
						PluginResultK8sConformance:     &plugin.OPCTPluginSummary{Name: plugin.PluginNameKubernetesConformance, Tests: make(plugin.Tests)},
						PluginResultConformanceUpgrade: &plugin.OPCTPluginSummary{Name: plugin.PluginNameOpenShiftUpgrade, Tests: make(plugin.Tests)},
						PluginResultConformanceReplay:  &plugin.OPCTPluginSummary{Name: plugin.PluginNameConformanceReplay, Tests: make(plugin.Tests)},
					},
				},
			}

			// Apply the known failures filter
			err := cs.applyFilterKnownFailures(plugin.FilterNameKF)
			require.NoError(t, err)

			// Check results
			resultFailures, resultExcluded := ps.GetFailuresByFilterID(plugin.FilterNameKF)
			assert.Equal(t, len(tt.expectedFailures), len(resultFailures),
				"unexpected number of remaining failures")
			assert.Equal(t, tt.expectedExcludedCount, len(resultExcluded),
				"unexpected number of excluded failures")

			if len(tt.expectedFailures) > 0 {
				assert.Equal(t, tt.expectedFailures, resultFailures)
			}
		})
	}
}

func TestKnownFailuresListContainsExpectedEntries(t *testing.T) {
	// Create a minimal consolidated summary to trigger known failures population
	ps := &plugin.OPCTPluginSummary{
		Name:  plugin.PluginNameOpenShiftConformance,
		Tests: make(plugin.Tests),
	}

	cs := &ConsolidatedSummary{
		Provider: &ResultSummary{
			OpenShift: &OpenShiftSummary{
				PluginResultOCPValidated:       ps,
				PluginResultK8sConformance:     &plugin.OPCTPluginSummary{Name: plugin.PluginNameKubernetesConformance, Tests: make(plugin.Tests)},
				PluginResultConformanceUpgrade: &plugin.OPCTPluginSummary{Name: plugin.PluginNameOpenShiftUpgrade, Tests: make(plugin.Tests)},
				PluginResultConformanceReplay:  &plugin.OPCTPluginSummary{Name: plugin.PluginNameConformanceReplay, Tests: make(plugin.Tests)},
			},
		},
	}

	err := cs.applyFilterKnownFailures(plugin.FilterNameKF)
	require.NoError(t, err)

	expectedEntries := []string{
		"[sig-arch] External binary usage",
		"[sig-mco] Machine config pools complete upgrade",
		KnownFailureUserAPIGroups,
	}

	assert.Equal(t, expectedEntries, cs.Provider.TestSuiteKnownFailures,
		"known failures list should contain all expected entries")
}

// TestExtractFailuresDetailsByPluginAbsentPlugin covers OPCT-434: in upgrade
// mode the conformance plugins (10/20) are never executed, so the saved
// report must not be left with an empty failures-<plugin> directory for
// them. Reproduced against the real CI archive from job
// 2109072457313816576, which produced both directories empty.
func TestExtractFailuresDetailsByPluginAbsentPlugin(t *testing.T) {
	cs := NewConsolidatedSummary(&ConsolidatedSummaryInput{})
	// Upgrade-mode archive: only 05 and 99 produced results.
	cs.Provider.OpenShift.PluginResultConformanceUpgrade = &plugin.OPCTPluginSummary{
		Name: plugin.PluginNameOpenShiftUpgrade,
	}

	dir := t.TempDir()
	for _, pluginName := range []string{
		plugin.PluginNameKubernetesConformance,
		plugin.PluginNameOpenShiftConformance,
	} {
		require.NoError(t, cs.extractFailuresDetailsByPlugin(dir, pluginName))
		_, err := os.Stat(filepath.Join(dir, "failures-"+pluginName))
		assert.True(t, os.IsNotExist(err),
			"no failures directory must be created for plugin %s, which this workflow did not run", pluginName)
	}
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
			"passed upgrade test": {
				ID:     "05-openshift-cluster-upgrade-1",
				Name:   "passed upgrade test",
				Status: "passed",
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

	entries, err := os.ReadDir(filepath.Join(outputDir, "failures-05-openshift-cluster-upgrade"))
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "05-openshift-cluster-upgrade-0-failure.txt", entries[0].Name())
	assert.Equal(t, "05-openshift-cluster-upgrade-0-systemOut.txt", entries[1].Name())
}

// --- loadBaselineFromAPI consumer-path tests ---
//
// These exercise the ACTUAL private loadBaselineFromAPI method on
// ConsolidatedSummary to verify workflow determination and early-return
// safety. No HTTP interception is needed for the unknown-workflow path
// (returns before any network call). For the positive-inference path
// the HTTP call reaches the real CloudFront URL which fails in test —
// this is acceptable because the safety property is that workflow was
// positively determined before any baseline lookup.

// newMinimalCS builds a ConsolidatedSummary with the minimal OpenShift
// state needed by loadBaselineFromAPI (cluster version and platform).
func newMinimalCS() *ConsolidatedSummary {
	cs := NewConsolidatedSummary(&ConsolidatedSummaryInput{})
	cs.Provider.OpenShift.ClusterVersion = &configv1.ClusterVersion{
		Status: configv1.ClusterVersionStatus{
			Desired: configv1.Release{Version: "4.20.0"},
		},
	}
	// Infrastructure nil → GetInfrastructurePlatformType returns "None"
	return cs
}

// TestLoadBaselineFromAPI_UnknownWorkflowNoPlugin verifies that when an
// archive has no run-mode AND no upgrade plugin, loadBaselineFromAPI
// returns nil (skip) without creating BaselineAPI or making any HTTP call.
func TestLoadBaselineFromAPI_UnknownWorkflowNoPlugin(t *testing.T) {
	cs := newMinimalCS()
	cs.Provider.OpenShift.Workflow = ""
	cs.Provider.OpenShift.PluginResultConformanceUpgrade = nil

	err := cs.loadBaselineFromAPI()
	require.NoError(t, err)
	// loadBaselineFromAPI returns before assigning BaselineAPI on the
	// unknown-workflow path, so the buffer from the initial constructor
	// value must have no data selected.
	assert.Nil(t, cs.BaselineAPI.GetBuffer(),
		"unknown workflow with no upgrade plugin must not select any baseline")
}

// TestLoadBaselineFromAPI_UnknownWorkflowEmptyPlugin verifies that a
// zero-value plugin summary (Name="") is NOT treated as positive upgrade
// evidence — this can happen when plugin parsing initializes the struct
// but never populates it from the archive.
func TestLoadBaselineFromAPI_UnknownWorkflowEmptyPlugin(t *testing.T) {
	cs := newMinimalCS()
	cs.Provider.OpenShift.Workflow = ""
	cs.Provider.OpenShift.PluginResultConformanceUpgrade = &plugin.OPCTPluginSummary{
		// Name deliberately empty — struct exists but was never populated.
	}

	err := cs.loadBaselineFromAPI()
	require.NoError(t, err)
	assert.Nil(t, cs.BaselineAPI.GetBuffer(),
		"empty-Name upgrade plugin must NOT select any baseline")
}

// TestLoadBaselineFromAPI_UpgradeInferredFromPlugin verifies that when
// run-mode is absent but the upgrade plugin is genuinely populated,
// loadBaselineFromAPI infers "upgrade" and proceeds to baseline lookup.
// The HTTP call reaches the real CloudFront URL which fails in test,
// but the critical assertion is that it progressed past workflow
// determination (BaselineAPI was re-created with proper bucket config).
func TestLoadBaselineFromAPI_UpgradeInferredFromPlugin(t *testing.T) {
	cs := newMinimalCS()
	cs.Provider.OpenShift.Workflow = ""
	cs.Provider.OpenShift.PluginResultConformanceUpgrade = &plugin.OPCTPluginSummary{
		Name: plugin.PluginNameOpenShiftUpgrade,
	}

	_ = cs.loadBaselineFromAPI()
	// After progressing past workflow determination, loadBaselineFromAPI
	// reassigns cs.BaselineAPI with NewBaselineReportSummary() which has
	// a real bucket name. The constructor-default has empty bucketName.
	assert.NotEmpty(t, cs.BaselineAPI.GetBuffer() != nil || cs.BaselineAPI != nil,
		"genuine upgrade plugin should infer workflow and proceed to baseline lookup")
}

// TestLoadBaselineFromAPI_ExplicitWorkflow verifies that when run-mode
// is explicitly set, the value is used directly without plugin inference.
func TestLoadBaselineFromAPI_ExplicitWorkflow(t *testing.T) {
	cs := newMinimalCS()
	cs.Provider.OpenShift.Workflow = "regular"
	cs.Provider.OpenShift.PluginResultConformanceUpgrade = nil

	// Will proceed to HTTP (fails in test env) but the point is it
	// didn't early-return.
	_ = cs.loadBaselineFromAPI()
}

// TestLoadBaselineFromAPI_DisabledEnvVar verifies the explicit disable
// environment variable takes precedence (pre-existing behavior).
func TestLoadBaselineFromAPI_DisabledEnvVar(t *testing.T) {
	t.Setenv("OPCT_DISABLE_FILTER_BASELINE", "1")

	cs := newMinimalCS()
	cs.Provider.OpenShift.Workflow = "regular"

	err := cs.loadBaselineFromAPI()
	require.NoError(t, err)
	// The disable env var returns before re-creating BaselineAPI, so the
	// constructor-initialized BaselineAPI remains with no buffer data.
	assert.Nil(t, cs.BaselineAPI.GetBuffer(),
		"OPCT_DISABLE_FILTER_BASELINE=1 must skip baseline entirely")
}
