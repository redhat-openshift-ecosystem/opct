package summary

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
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
// These exercise the ACTUAL private loadBaselineFromAPI method using an
// httptest.Server for deterministic HTTP. The testBaselineURL field on
// ConsolidatedSummary redirects API calls to the test server via the
// WithBaseURL seam on BaselineConfig. Tests make ZERO external network
// calls. Each test gets its own server; no global state mutation.

// requestRecorder is an httptest handler that records request paths and
// serves canned responses. Unregistered paths return 404.
type requestRecorder struct {
	mu       sync.Mutex
	paths    []string
	handlers map[string][]byte // URL path → JSON body
}

func newRecorder(handlers map[string][]byte) *requestRecorder {
	if handlers == nil {
		handlers = map[string][]byte{}
	}
	return &requestRecorder{handlers: handlers}
}

func (rr *requestRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rr.mu.Lock()
	rr.paths = append(rr.paths, r.URL.Path)
	rr.mu.Unlock()

	if body, ok := rr.handlers[r.URL.Path]; ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
		return
	}
	http.NotFound(w, r)
}

func (rr *requestRecorder) requestedPaths() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	cp := make([]string, len(rr.paths))
	copy(cp, rr.paths)
	return cp
}

func (rr *requestRecorder) requestCount() int {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return len(rr.paths)
}

// makeBaselinePayload builds a minimal valid baseline JSON with the given
// workflow in setup.api.workflow.
func makeBaselinePayload(workflow string) []byte {
	api := map[string]interface{}{
		"openshiftRelease": "4.20",
		"platformType":     "None",
		"executionDate":    "2026-01-01T00:00:00Z",
		"dataPath":         "4.20_None_20260101.json",
	}
	if workflow != "" {
		api["workflow"] = workflow
	}
	obj := map[string]interface{}{
		"setup":    map[string]interface{}{"api": api},
		"provider": map[string]interface{}{"plugins": map[string]interface{}{}},
	}
	b, _ := json.Marshal(obj)
	return b
}

// newMinimalCS builds a ConsolidatedSummary with the minimal OpenShift
// state needed by loadBaselineFromAPI (cluster version and platform).
// Infrastructure nil → GetInfrastructurePlatformType returns "None".
func newMinimalCS() *ConsolidatedSummary {
	cs := NewConsolidatedSummary(&ConsolidatedSummaryInput{})
	cs.Provider.OpenShift.ClusterVersion = &configv1.ClusterVersion{
		Status: configv1.ClusterVersionStatus{
			Desired: configv1.Release{Version: "4.20.0"},
		},
	}
	return cs
}

// newMinimalCSWithServer creates a ConsolidatedSummary and httptest server.
// The CS's testBaselineURL is set to the server URL. Server is closed via
// t.Cleanup.
func newMinimalCSWithServer(t *testing.T, rr *requestRecorder) *ConsolidatedSummary {
	srv := httptest.NewServer(rr)
	t.Cleanup(srv.Close)
	cs := newMinimalCS()
	cs.testBaselineURL = srv.URL
	return cs
}

// TestLoadBaselineFromAPI_UnknownWorkflowNoPlugin — archive has no
// run-mode AND no upgrade plugin. Must return nil (skip), make ZERO
// HTTP requests, and select no baseline.
func TestLoadBaselineFromAPI_UnknownWorkflowNoPlugin(t *testing.T) {
	rr := newRecorder(nil)
	cs := newMinimalCSWithServer(t, rr)
	cs.Provider.OpenShift.Workflow = ""
	cs.Provider.OpenShift.PluginResultConformanceUpgrade = nil

	err := cs.loadBaselineFromAPI()
	require.NoError(t, err)
	assert.Equal(t, 0, rr.requestCount(),
		"unknown workflow must make zero HTTP requests")
	assert.Nil(t, cs.BaselineAPI.GetBuffer(),
		"unknown workflow must not select any baseline")
}

// TestLoadBaselineFromAPI_UnknownWorkflowEmptyPlugin — struct exists
// but Name="" (never populated). Must NOT treat as positive evidence.
func TestLoadBaselineFromAPI_UnknownWorkflowEmptyPlugin(t *testing.T) {
	rr := newRecorder(nil)
	cs := newMinimalCSWithServer(t, rr)
	cs.Provider.OpenShift.Workflow = ""
	cs.Provider.OpenShift.PluginResultConformanceUpgrade = &plugin.OPCTPluginSummary{}

	err := cs.loadBaselineFromAPI()
	require.NoError(t, err)
	assert.Equal(t, 0, rr.requestCount(),
		"empty-Name plugin must make zero HTTP requests")
	assert.Nil(t, cs.BaselineAPI.GetBuffer(),
		"empty-Name plugin must not select any baseline")
}

// TestLoadBaselineFromAPI_UpgradeInferredFromPlugin — no run-mode but
// genuine upgrade plugin. Must infer "upgrade", request upgrade-scoped
// URL first, and accept matching payload.
func TestLoadBaselineFromAPI_UpgradeInferredFromPlugin(t *testing.T) {
	rr := newRecorder(map[string][]byte{
		"/result/summary/4.20_None_upgrade_latest.json": makeBaselinePayload("upgrade"),
	})
	cs := newMinimalCSWithServer(t, rr)
	cs.Provider.OpenShift.Workflow = ""
	cs.Provider.OpenShift.PluginResultConformanceUpgrade = &plugin.OPCTPluginSummary{
		Name: plugin.PluginNameOpenShiftUpgrade,
	}

	err := cs.loadBaselineFromAPI()
	require.NoError(t, err)

	// Must have requested the upgrade-scoped path first.
	paths := rr.requestedPaths()
	require.NotEmpty(t, paths)
	assert.Equal(t, "/result/summary/4.20_None_upgrade_latest.json", paths[0],
		"first request must be for upgrade-scoped path")

	// Must have selected the matching payload.
	buf := cs.BaselineAPI.GetBuffer()
	require.NotNil(t, buf, "must select a baseline when upgrade inferred")
	wf, ok := buf.InferWorkflow()
	require.True(t, ok)
	assert.Equal(t, "upgrade", wf,
		"selected baseline must carry workflow=upgrade")
}

// TestLoadBaselineFromAPI_ExplicitRegular — run-mode is "regular". Must
// request regular-scoped URL and accept matching payload.
func TestLoadBaselineFromAPI_ExplicitRegular(t *testing.T) {
	rr := newRecorder(map[string][]byte{
		"/result/summary/4.20_None_regular_latest.json": makeBaselinePayload("regular"),
	})
	cs := newMinimalCSWithServer(t, rr)
	cs.Provider.OpenShift.Workflow = "regular"

	err := cs.loadBaselineFromAPI()
	require.NoError(t, err)

	paths := rr.requestedPaths()
	require.NotEmpty(t, paths)
	assert.Equal(t, "/result/summary/4.20_None_regular_latest.json", paths[0],
		"first request must be for regular-scoped path")

	buf := cs.BaselineAPI.GetBuffer()
	require.NotNil(t, buf, "must select a baseline for explicit regular")
	wf, ok := buf.InferWorkflow()
	require.True(t, ok)
	assert.Equal(t, "regular", wf)
}

// TestLoadBaselineFromAPI_DisabledEnvVar — OPCT_DISABLE_FILTER_BASELINE=1
// must skip entirely with zero HTTP requests.
func TestLoadBaselineFromAPI_DisabledEnvVar(t *testing.T) {
	t.Setenv("OPCT_DISABLE_FILTER_BASELINE", "1")

	rr := newRecorder(map[string][]byte{
		"/result/summary/4.20_None_regular_latest.json": makeBaselinePayload("regular"),
	})
	cs := newMinimalCSWithServer(t, rr)
	cs.Provider.OpenShift.Workflow = "regular"

	err := cs.loadBaselineFromAPI()
	require.NoError(t, err)
	assert.Equal(t, 0, rr.requestCount(),
		"disabled env var must make zero HTTP requests")
	assert.Nil(t, cs.BaselineAPI.GetBuffer(),
		"disabled env var must not select any baseline")
}

// TestLoadBaselineFromAPI_RejectsMismatchedPayload — every path serves
// "regular" but upgrade is inferred. Must reject all and select nothing.
func TestLoadBaselineFromAPI_RejectsMismatchedPayload(t *testing.T) {
	wrongPayload := makeBaselinePayload("regular")
	rr := newRecorder(map[string][]byte{
		"/result/summary/4.20_None_upgrade_latest.json": wrongPayload,
		"/result/summary/4.20_None_latest.json":         wrongPayload,
		"/result/summary/4.20_AWS_upgrade_latest.json":  wrongPayload,
		"/result/summary/4.20_AWS_latest.json":          wrongPayload,
	})
	cs := newMinimalCSWithServer(t, rr)
	cs.Provider.OpenShift.Workflow = ""
	cs.Provider.OpenShift.PluginResultConformanceUpgrade = &plugin.OPCTPluginSummary{
		Name: plugin.PluginNameOpenShiftUpgrade,
	}

	err := cs.loadBaselineFromAPI()
	require.NoError(t, err)
	assert.Nil(t, cs.BaselineAPI.GetBuffer(),
		"must not select baseline when all payloads carry wrong workflow")
	assert.Greater(t, rr.requestCount(), 0,
		"should have attempted HTTP requests before rejecting")
}
