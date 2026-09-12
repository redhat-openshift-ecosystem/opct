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
