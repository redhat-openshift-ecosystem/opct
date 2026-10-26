package baseline

import (
	"bytes"
	"testing"

	"github.com/redhat-openshift-ecosystem/opct/internal/report"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogs redirects the logrus standard logger for the duration of fn.
// The rejection reasons are only surfaced through logs, so asserting on them
// is the only way to verify an operator can tell why a baseline was refused.
func captureLogs(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	orig := log.StandardLogger().Out
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })
	fn()
	return buf.String()
}

func sloOutput(id string) *report.SLOOutput {
	return &report.SLOOutput{
		ID:        id,
		SLO:       "slo-" + id,
		SLITarget: "100%",
		SLIActual: "90%",
	}
}

func TestBaselineArchiveHasrequiredChecks(t *testing.T) {
	t.Run("no failed checks is publishable", func(t *testing.T) {
		re := &report.ReportData{Checks: &report.ReportChecks{
			Pass: []*report.SLOOutput{sloOutput(report.CheckID004)},
		}}
		assert.True(t, baselineArchiveHasrequiredChecks(re),
			"only the Fail bucket may reject a baseline")
	})

	t.Run("empty check buckets are publishable", func(t *testing.T) {
		re := &report.ReportData{Checks: &report.ReportChecks{}}
		assert.True(t, baselineArchiveHasrequiredChecks(re))
	})

	t.Run("a non-required failed check does not reject", func(t *testing.T) {
		re := &report.ReportData{Checks: &report.ReportChecks{
			Fail: []*report.SLOOutput{sloOutput(report.CheckID001), sloOutput(report.CheckID023A)},
		}}
		assert.True(t, baselineArchiveHasrequiredChecks(re),
			"only OPCT-004, OPCT-005 and OPCT-022 gate the publication")
	})

	// Each required check is gated individually: a regression that drops one
	// of the three from the condition must not hide behind the other two.
	for _, id := range []string{report.CheckID004, report.CheckID005, report.CheckID022} {
		t.Run("required check "+id+" rejects", func(t *testing.T) {
			re := &report.ReportData{Checks: &report.ReportChecks{
				Fail: []*report.SLOOutput{sloOutput(id)},
			}}
			var got bool
			logs := captureLogs(t, func() {
				got = baselineArchiveHasrequiredChecks(re)
			})
			assert.False(t, got)
			assert.Contains(t, logs, id, "the rejecting check must be identifiable from the logs")
			assert.Contains(t, logs, "100%", "the target value must be reported")
			assert.Contains(t, logs, "90%", "the actual value must be reported")
		})
	}

	t.Run("every offending check is reported, not just the first", func(t *testing.T) {
		re := &report.ReportData{Checks: &report.ReportChecks{
			Fail: []*report.SLOOutput{
				sloOutput(report.CheckID004),
				sloOutput(report.CheckID001),
				sloOutput(report.CheckID022),
			},
		}}
		var got bool
		logs := captureLogs(t, func() {
			got = baselineArchiveHasrequiredChecks(re)
		})
		assert.False(t, got)
		assert.Contains(t, logs, report.CheckID004)
		assert.Contains(t, logs, report.CheckID022,
			"a single run must list all the reasons, so the archive is not fixed one check at a time")
		assert.NotContains(t, logs, report.CheckID001,
			"a check that does not gate publication must not be logged as a rejection")
	})

	t.Run("message is appended when present", func(t *testing.T) {
		check := sloOutput(report.CheckID005)
		check.Message = "etcd fsync above threshold"
		re := &report.ReportData{Checks: &report.ReportChecks{
			Fail: []*report.SLOOutput{check},
		}}
		logs := captureLogs(t, func() {
			assert.False(t, baselineArchiveHasrequiredChecks(re))
		})
		assert.Contains(t, logs, "message=")
		assert.Contains(t, logs, "etcd fsync above threshold")
	})

	t.Run("message is omitted when empty", func(t *testing.T) {
		re := &report.ReportData{Checks: &report.ReportChecks{
			Fail: []*report.SLOOutput{sloOutput(report.CheckID005)},
		}}
		logs := captureLogs(t, func() {
			assert.False(t, baselineArchiveHasrequiredChecks(re))
		})
		assert.NotContains(t, logs, "message=")
	})
}

// TestBaselineArchiveHasrequiredChecksFailsClosed covers the inputs reachable
// when report processing fails: baselinePublishCmdRun only logs the error from
// Populate and keeps going, so this function can be reached with the checks
// never evaluated. Publishing then would seed the review process with an
// unvetted baseline.
func TestBaselineArchiveHasrequiredChecksFailsClosed(t *testing.T) {
	t.Run("nil report", func(t *testing.T) {
		var got bool
		logs := captureLogs(t, func() {
			got = baselineArchiveHasrequiredChecks(nil)
		})
		assert.False(t, got)
		assert.Contains(t, logs, "checks were not evaluated")
	})

	t.Run("report without evaluated checks", func(t *testing.T) {
		var got bool
		logs := captureLogs(t, func() {
			got = baselineArchiveHasrequiredChecks(&report.ReportData{})
		})
		assert.False(t, got)
		assert.Contains(t, logs, "checks were not evaluated")
	})

	t.Run("nil entry in the failed list is skipped", func(t *testing.T) {
		re := &report.ReportData{Checks: &report.ReportChecks{
			Fail: []*report.SLOOutput{nil, sloOutput(report.CheckID004)},
		}}
		var got bool
		require.NotPanics(t, func() {
			got = baselineArchiveHasrequiredChecks(re)
		})
		assert.False(t, got, "a nil entry must not mask a real rejection later in the list")
	})
}
