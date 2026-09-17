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

func TestApplyFilterKnownFailures(t *testing.T) {
	// userAPITestName aliases the exported constant for brevity in test cases.
	const userAPITestName = KnownFailureUserAPIGroups

	// SA-specific failure message that is the expected false positive.
	const saFailureMsg = "unexpected groups returned for user/~: got [system:authenticated system:serviceaccounts system:serviceaccounts:opct]"

	tests := []struct {
		name                  string
		inputFailures         []string
		failureMessages       map[string]string // test name → Failure field content
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
