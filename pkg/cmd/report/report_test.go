package report

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	internalreport "github.com/redhat-openshift-ecosystem/opct/internal/report"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShowErrorDetailsUsesAvailableConformancePlugins(t *testing.T) {
	const failedTest = "[sig-trt] Skipped annotations present"
	reportData := &internalreport.ReportData{Provider: &internalreport.ReportResult{
		Plugins: map[string]*internalreport.ReportPlugin{
			plugin.PluginNameOpenShiftUpgrade: {
				Name: plugin.PluginNameOpenShiftUpgrade,
				Tests: plugin.Tests{failedTest: {
					Name:   failedTest,
					Status: "failed",
				}},
				FailedFiltered: []*internalreport.ReportTestFailure{{Name: failedTest}},
			},
			plugin.PluginNameArtifactsCollector: {Name: plugin.PluginNameArtifactsCollector},
		},
	}}

	var logOutput bytes.Buffer
	originalLogOutput := log.StandardLogger().Out
	log.SetOutput(&logOutput)
	t.Cleanup(func() { log.SetOutput(originalLogOutput) })

	output := captureStdout(t, func() {
		require.NoError(t, showErrorDetails(reportData, false))
	})

	assert.Contains(t, output, "==> 05-openshift-cluster-upgrade - test failures:")
	assert.Contains(t, output, "Failed tests to review")
	assert.NotContains(t, output, "99-openshift-artifacts-collector - test failures")
	assert.NotContains(t, logOutput.String(), "unable to get plugin")
}

func TestShowErrorDetailPluginNamesUnavailablePlugin(t *testing.T) {
	var logOutput bytes.Buffer
	originalLogOutput := log.StandardLogger().Out
	log.SetOutput(&logOutput)
	t.Cleanup(func() { log.SetOutput(originalLogOutput) })

	showErrorDetailPlugin(plugin.PluginNameKubernetesConformance, nil, false)

	assert.Contains(t, logOutput.String(), "unable to get plugin 10-openshift-kube-conformance")
}

func TestShowErrorDetailsSkipsSkippedPlugin(t *testing.T) {
	reportData := &internalreport.ReportData{Provider: &internalreport.ReportResult{
		Plugins: map[string]*internalreport.ReportPlugin{
			plugin.PluginNameKubernetesConformance: {
				Name: plugin.PluginNameKubernetesConformance,
				Stat: &internalreport.ReportPluginStat{Status: "skipped"},
			},
		},
	}}

	var logOutput bytes.Buffer
	originalLogOutput := log.StandardLogger().Out
	log.SetOutput(&logOutput)
	t.Cleanup(func() { log.SetOutput(originalLogOutput) })

	output := captureStdout(t, func() {
		require.NoError(t, showErrorDetails(reportData, false))
	})

	assert.NotContains(t, output, "10-openshift-kube-conformance - test failures")
	assert.NotContains(t, logOutput.String(), "unable to get plugin")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = originalStdout })

	fn()
	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	return string(output)
}
