package summary

import (
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
