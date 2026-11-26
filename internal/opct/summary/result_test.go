package summary

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmware-tanzu/sonobuoy/pkg/client/results"
)

func TestProcessPluginResultKeepsFailedDuplicateTest(t *testing.T) {
	const testName = "[sig-trt] Skipped annotations present"
	const failure = "upgrade failure output"
	const systemOut = "upgrade system output"

	cs := NewConsolidatedSummary(&ConsolidatedSummaryInput{})
	err := cs.Provider.processPluginResult(&results.Item{
		Name:   plugin.PluginNameOpenShiftUpgrade,
		Status: results.StatusFailed,
		Items: []results.Item{
			{
				Name:   testName,
				Status: results.StatusFailed,
				Details: map[string]interface{}{
					"failure":    failure,
					"system-out": systemOut,
				},
			},
			{Name: testName, Status: results.StatusPassed},
		},
	})
	require.NoError(t, err)

	upgrade := cs.Provider.OpenShift.GetResultConformanceUpgrade()
	require.NotNil(t, upgrade)
	assert.Equal(t, int64(2), upgrade.Total)
	assert.Equal(t, int64(1), upgrade.Passed)
	assert.Equal(t, int64(1), upgrade.Failed)
	assert.Equal(t, []string{testName}, upgrade.FailedList)
	require.Len(t, upgrade.Tests, 1)
	assert.Equal(t, results.StatusFailed, upgrade.Tests[testName].Status)
	assert.Equal(t, failure, upgrade.Tests[testName].Failure)
	assert.Equal(t, systemOut, upgrade.Tests[testName].SystemOut)
}

// TestProcessPluginResultDeduplicatesRepeatedFailure covers the case where the
// same test name fails more than once in a result file. FailedList drives
// extractSaveTestErrors, which opens the detail files with O_APPEND, so a
// duplicated entry would write the same failure text into the file twice.
func TestProcessPluginResultDeduplicatesRepeatedFailure(t *testing.T) {
	const testName = "[sig-arch] Cluster should remain functional during upgrade"

	cs := NewConsolidatedSummary(&ConsolidatedSummaryInput{})
	err := cs.Provider.processPluginResult(&results.Item{
		Name:   plugin.PluginNameOpenShiftUpgrade,
		Status: results.StatusFailed,
		Items: []results.Item{
			{
				Name:    testName,
				Status:  results.StatusFailed,
				Details: map[string]interface{}{"failure": "first failure"},
			},
			{
				Name:    testName,
				Status:  results.StatusTimeout,
				Details: map[string]interface{}{"failure": "second failure"},
			},
		},
	})
	require.NoError(t, err)

	upgrade := cs.Provider.OpenShift.GetResultConformanceUpgrade()
	require.NotNil(t, upgrade)
	assert.Equal(t, []string{testName}, upgrade.FailedList,
		"a repeated failure must contribute a single FailedList entry, or its detail files are written twice")
	require.Len(t, upgrade.Tests, 1)
	assert.Equal(t, results.StatusFailed, upgrade.Tests[testName].Status,
		"the first failing occurrence must be kept")
	assert.Equal(t, "first failure", upgrade.Tests[testName].Failure)
	assert.Equal(t, plugin.PluginNameOpenShiftUpgrade+"-0", upgrade.Tests[testName].ID,
		"the retained ID must be the one the extracted failure artifacts are named after")
}

// TestProcessPluginResultLaterFailureReplacesPass is the inverse ordering: a
// passing occurrence recorded first must not mask a later failure.
func TestProcessPluginResultLaterFailureReplacesPass(t *testing.T) {
	const testName = "[sig-network] pods should be reachable"

	cs := NewConsolidatedSummary(&ConsolidatedSummaryInput{})
	err := cs.Provider.processPluginResult(&results.Item{
		Name:   plugin.PluginNameOpenShiftUpgrade,
		Status: results.StatusFailed,
		Items: []results.Item{
			{Name: testName, Status: results.StatusPassed},
			{
				Name:    testName,
				Status:  results.StatusFailed,
				Details: map[string]interface{}{"failure": "late failure"},
			},
			{Name: testName, Status: results.StatusPassed},
		},
	})
	require.NoError(t, err)

	upgrade := cs.Provider.OpenShift.GetResultConformanceUpgrade()
	require.NotNil(t, upgrade)
	assert.Equal(t, []string{testName}, upgrade.FailedList)
	require.Len(t, upgrade.Tests, 1)
	assert.Equal(t, results.StatusFailed, upgrade.Tests[testName].Status)
	assert.Equal(t, "late failure", upgrade.Tests[testName].Failure)
}

// writePluginJunit creates a minimal results directory for a plugin with the
// given JUnit file content, mimicking the layout of a sonobuoy archive.
func writePluginJunit(t *testing.T, pluginName, junit string) string {
	t.Helper()
	dir := t.TempDir()
	junitDir := filepath.Join(dir, "plugins", pluginName, "results", "global")
	require.NoError(t, os.MkdirAll(junitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(junitDir, "junit_e2e__test.xml"), []byte(junit), 0o644))
	return dir
}

// TestProcessPluginResultJunitOverridesStaleSuiteStatus covers OPCT-372: the
// JUnit file keeps a stale testsuite failures="1" attribute after a flaky test
// was retried and passed. The sonobuoy item status is derived from that
// attribute, so the summary must re-count from the test cases instead and not
// report "Status: failed" with "Failed: 0".
func TestProcessPluginResultJunitOverridesStaleSuiteStatus(t *testing.T) {
	dir := writePluginJunit(t, plugin.PluginNameOpenShiftUpgrade, `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="openshift-tests-upgrade" tests="3" skipped="1" failures="1" time="1">
  <properties></properties>
  <testcase name="[sig-arch] test a" time="1"></testcase>
  <testcase name="[sig-arch] test b" time="1"><skipped message="skipped"/></testcase>
  <testcase name="[sig-trt] Skipped annotations present" time="1"></testcase>
</testsuite>`)

	cs := NewConsolidatedSummary(&ConsolidatedSummaryInput{})
	cs.Provider.reader = results.NewReaderFromDir(dir)
	err := cs.Provider.processPluginResult(&results.Item{
		Name:   plugin.PluginNameOpenShiftUpgrade,
		Status: results.StatusFailed,
		Items: []results.Item{
			{Name: "[sig-arch] test a", Status: results.StatusPassed},
			{Name: "[sig-arch] test b", Status: results.StatusSkipped},
			{Name: "[sig-trt] Skipped annotations present", Status: results.StatusPassed},
		},
	})
	require.NoError(t, err)

	upgrade := cs.Provider.OpenShift.GetResultConformanceUpgrade()
	require.NotNil(t, upgrade)
	assert.Equal(t, results.StatusPassed, upgrade.Status,
		"no failing test case in the JUnit file must not be reported as failed")
	assert.Equal(t, int64(3), upgrade.Total)
	assert.Equal(t, int64(2), upgrade.Passed)
	assert.Equal(t, int64(0), upgrade.Failed)
	assert.Equal(t, int64(1), upgrade.Skipped)
	assert.Empty(t, upgrade.FailedList)
}

// TestProcessPluginResultJunitCountsFailuresFromTestCases is the inverse
// mismatch: the counters must come from the test cases, marking the plugin
// failed when a test case failed even if other metadata disagrees.
func TestProcessPluginResultJunitCountsFailuresFromTestCases(t *testing.T) {
	dir := writePluginJunit(t, plugin.PluginNameOpenShiftUpgrade, `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="openshift-tests-upgrade" tests="2" skipped="0" failures="0" time="1">
  <properties></properties>
  <testcase name="[sig-arch] test a" time="1"></testcase>
  <testcase name="[sig-arch] test b" time="1"><failure type="Failure">boom</failure></testcase>
</testsuite>`)

	cs := NewConsolidatedSummary(&ConsolidatedSummaryInput{})
	cs.Provider.reader = results.NewReaderFromDir(dir)
	err := cs.Provider.processPluginResult(&results.Item{
		Name:   plugin.PluginNameOpenShiftUpgrade,
		Status: results.StatusPassed,
		Items: []results.Item{
			{Name: "[sig-arch] test a", Status: results.StatusPassed},
			{
				Name:    "[sig-arch] test b",
				Status:  results.StatusFailed,
				Details: map[string]interface{}{"failure": "boom"},
			},
		},
	})
	require.NoError(t, err)

	upgrade := cs.Provider.OpenShift.GetResultConformanceUpgrade()
	require.NotNil(t, upgrade)
	assert.Equal(t, results.StatusFailed, upgrade.Status)
	assert.Equal(t, int64(2), upgrade.Total)
	assert.Equal(t, int64(1), upgrade.Passed)
	assert.Equal(t, int64(1), upgrade.Failed)
	assert.Equal(t, []string{"[sig-arch] test b"}, upgrade.FailedList)
}

// TestProcessPluginResultFallsBackWhenJunitMissing covers plugins that do not
// produce JUnit files (for example 99-openshift-artifacts-collector): the
// sonobuoy results must keep driving the summary.
func TestProcessPluginResultFallsBackWhenJunitMissing(t *testing.T) {
	cs := NewConsolidatedSummary(&ConsolidatedSummaryInput{})
	cs.Provider.reader = results.NewReaderFromDir(t.TempDir())
	err := cs.Provider.processPluginResult(&results.Item{
		Name:   plugin.PluginNameOpenShiftUpgrade,
		Status: results.StatusFailed,
		Items: []results.Item{
			{
				Name:    "[sig-arch] test a",
				Status:  results.StatusFailed,
				Details: map[string]interface{}{"failure": "boom"},
			},
		},
	})
	require.NoError(t, err)

	upgrade := cs.Provider.OpenShift.GetResultConformanceUpgrade()
	require.NotNil(t, upgrade)
	assert.Equal(t, results.StatusFailed, upgrade.Status)
	assert.Equal(t, int64(1), upgrade.Total)
	assert.Equal(t, int64(1), upgrade.Failed)
}

// TestProcessPluginResultFallsBackWhenJunitBroken covers a JUnit file that
// exists but cannot be parsed (for example truncated by the aggregator): the
// sonobuoy results must keep driving the summary.
func TestProcessPluginResultFallsBackWhenJunitBroken(t *testing.T) {
	dir := writePluginJunit(t, plugin.PluginNameOpenShiftUpgrade, `<testsuite name="openshift-tests-upgrade" tests="1" skipped="0" failures="0"`)
	cs := NewConsolidatedSummary(&ConsolidatedSummaryInput{})
	cs.Provider.reader = results.NewReaderFromDir(dir)
	err := cs.Provider.processPluginResult(&results.Item{
		Name:   plugin.PluginNameOpenShiftUpgrade,
		Status: results.StatusFailed,
		Items: []results.Item{
			{
				Name:    "[sig-arch] test a",
				Status:  results.StatusFailed,
				Details: map[string]interface{}{"failure": "boom"},
			},
		},
	})
	require.NoError(t, err)

	upgrade := cs.Provider.OpenShift.GetResultConformanceUpgrade()
	require.NotNil(t, upgrade)
	assert.Equal(t, results.StatusFailed, upgrade.Status)
	assert.Equal(t, int64(1), upgrade.Total)
	assert.Equal(t, int64(1), upgrade.Failed)
}
