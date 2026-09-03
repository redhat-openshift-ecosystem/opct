package summary

import (
	"testing"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyFilterKnownFailures(t *testing.T) {
	// userAPITestName is the full test name for the UserAPI groups test
	// that is a known false-positive in OPCT environments using SA-based auth.
	// The test checks group membership of the authenticated user via users/~
	// and expects system:masters or system:cluster-admins, but ServiceAccounts
	// are always in system:serviceaccounts groups instead.
	const userAPITestName = "[sig-auth][Feature:UserAPI] users can manipulate groups " +
		"[apigroup:user.openshift.io][apigroup:authorization.openshift.io]" +
		"[apigroup:project.openshift.io] " +
		"[Suite:openshift/conformance/parallel]"

	tests := []struct {
		name                  string
		inputFailures         []string
		expectedFailures      []string
		expectedExcludedCount int
	}{
		{
			name: "should_exclude_all_known_failures",
			inputFailures: []string{
				"[sig-arch] External binary usage",
				"[sig-mco] Machine config pools complete upgrade",
				userAPITestName,
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
			expectedFailures: []string{
				"[sig-apps] some real failure test",
				"[sig-network] another real failure",
			},
			expectedExcludedCount: 1,
		},
		{
			name: "should_exclude_user_api_groups_test",
			inputFailures: []string{
				userAPITestName,
			},
			expectedFailures:      []string{},
			expectedExcludedCount: 1,
		},
		{
			name:                  "should_handle_no_failures",
			inputFailures:         []string{},
			expectedFailures:      []string{},
			expectedExcludedCount: 0,
		},
		{
			name: "should_keep_all_when_no_known_failures_match",
			inputFailures: []string{
				"[sig-apps] deployments should work",
				"[sig-network] services should be reachable",
			},
			expectedFailures: []string{
				"[sig-apps] deployments should work",
				"[sig-network] services should be reachable",
			},
			expectedExcludedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build test items map required by the filter (ps.Tests[v].State access)
			testItems := make(plugin.Tests, len(tt.inputFailures))
			for _, name := range tt.inputFailures {
				testItems[name] = &plugin.TestItem{
					Name:   name,
					Status: "failed",
					State:  "processed",
				}
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
		"[sig-auth][Feature:UserAPI] users can manipulate groups [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:project.openshift.io] [Suite:openshift/conformance/parallel]",
	}

	assert.Equal(t, expectedEntries, cs.Provider.TestSuiteKnownFailures,
		"known failures list should contain all expected entries")
}
