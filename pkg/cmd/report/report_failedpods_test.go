package report

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/summary"
	"github.com/redhat-openshift-ecosystem/opct/internal/report"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmware-tanzu/sonobuoy/pkg/discovery"
)

// failingWriter rejects every write, simulating a closed or broken stdout.
type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

// Golden outputs below were produced by the previous inline implementation
// in showReportAggregatedSummary; the rendered layout must stay the same.
func TestWriteFailedPodsOutput(t *testing.T) {
	t.Run("no failed pods", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, writeFailedPods(&buf, nil))
		assert.Equal(t, "\t\t\n", buf.String())
	})

	t.Run("failed pods", func(t *testing.T) {
		var buf bytes.Buffer
		pods := []discovery.HealthInfoDetails{
			{Namespace: "openshift-etcd", Name: "etcd-0", Healthy: false, Ready: "false", Reason: "CrashLoopBackOff", Message: "back-off restarting"},
			{Namespace: "ns", Name: "p", Healthy: true, Ready: "true"},
		}
		require.NoError(t, writeFailedPods(&buf, pods))
		assert.Equal(t, "\t\t\n Failed pods:\n"+
			"  Namespace/PodName\tHealthy\tReady\tReason\t\t\tMessage\n"+
			"  openshift-etcd/etcd-0\tfalse\tfalse\tCrashLoopBackOff\tback-off restarting\n"+
			"  ns/p\t\t\ttrue\ttrue\t\t\t\t\n", buf.String())
	})
}

func TestWriteFailedPodsReturnsWriteError(t *testing.T) {
	errWrite := errors.New("write failed")
	pods := []discovery.HealthInfoDetails{{Namespace: "ns", Name: "p", Ready: "false"}}

	assert.ErrorIs(t, writeFailedPods(failingWriter{err: errWrite}, pods), errWrite)
	assert.ErrorIs(t, writeFailedPods(failingWriter{err: errWrite}, nil), errWrite)
}

// The CLI summary is printed before the report is saved, so a failure writing
// the failed pods section to stdout must be logged and must not abort the
// report; otherwise --save-to/--save-only would not save the results.
func TestShowReportAggregatedSummaryWarnsOnFailedPodsWriteError(t *testing.T) {
	var logBuf bytes.Buffer
	origLog := log.StandardLogger().Out
	log.SetOutput(&logBuf)
	defer log.SetOutput(origLog)

	// Every write to a file opened read-only fails (EBADF), like a closed
	// or broken stdout.
	readOnly, err := os.Open(os.DevNull)
	require.NoError(t, err)
	defer func() { assert.NoError(t, readOnly.Close()) }()
	origStdout := os.Stdout
	os.Stdout = readOnly
	defer func() { os.Stdout = origStdout }()

	re := &report.ReportData{
		Summary: &report.ReportSummary{Tests: &report.ReportSummaryTests{Archive: "archive.tar.gz"}},
		Provider: &report.ReportResult{
			Version:          &report.ReportVersion{OpenShift: &summary.SummaryClusterVersionOutput{}},
			Infra:            &report.ReportInfra{},
			ClusterOperators: &report.ReportClusterOperators{},
			ClusterHealth: &report.ReportClusterHealth{
				PodHealthDetails: []discovery.HealthInfoDetails{{Namespace: "ns", Name: "p", Ready: "false"}},
			},
		},
	}

	assert.NoError(t, showReportAggregatedSummary(re))
	assert.Contains(t, logBuf.String(), "unable to write failed pods section")
}
