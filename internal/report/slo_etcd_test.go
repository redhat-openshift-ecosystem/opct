package report

import (
	"fmt"
	"testing"

	"github.com/redhat-openshift-ecosystem/opct/internal/openshift/mustgather"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newEtcdSlowReport builds a ReportData exposing a single parsed etcd
// slow-request population, formatted the way the must-gather parser emits it.
func newEtcdSlowReport(total, elevated, severe int) *ReportData {
	pct := func(v int) string {
		if total == 0 {
			return fmt.Sprintf("%d (0.000%%)", v)
		}
		return fmt.Sprintf("%d (%.3f%%)", v, (float64(v)/float64(total))*100)
	}
	return &ReportData{
		Provider: &ReportResult{
			MustGatherInfo: &mustgather.MustGather{
				ErrorEtcdLogs: &mustgather.ErrorEtcdLogs{
					FilterRequestSlowAll: map[string]*mustgather.BucketFilterStat{
						"all": {
							RequestCount: int64(total),
							StatCount:    fmt.Sprintf("%d", total),
							Higher500ms:  pct(elevated),
							Buckets: map[string]string{
								mustgather.BucketRangeName1000Inf: pct(severe),
								mustgather.BucketRangeNameAll:     fmt.Sprintf("%d ", total),
							},
						},
					},
				},
			},
		},
	}
}

// runEtcdCheck locates a check by ID and returns its evaluated result.
func runEtcdCheck(t *testing.T, re *ReportData, id string) CheckResult {
	t.Helper()
	for _, check := range NewCheckSummary(re).Checks {
		if check.ID == id {
			return check.Test()
		}
	}
	require.FailNowf(t, "check not found", "id=%s", id)
	return CheckResult{}
}

func TestCheckOPCT010A_ElevatedSlowRequests(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		total, elevated, severe int
		want                    CheckResultName
	}{
		{"no elevated events", 50, 0, 0, CheckResultNamePass},
		{"at absolute pass ceiling", 50, 10, 0, CheckResultNamePass},
		{"above count but within ratio", 100, 11, 0, CheckResultNamePass},
		{"exactly at ratio ceiling", 50, 20, 0, CheckResultNamePass},
		{"above count and ratio", 50, 21, 0, CheckResultNameWarn},
		{"predominantly elevated", 64, 39, 13, CheckResultNameWarn},
		{"sample below minimum", 9, 9, 9, CheckResultNameSkip},
		{"sample exactly at minimum", 10, 10, 0, CheckResultNamePass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := runEtcdCheck(t, newEtcdSlowReport(tc.total, tc.elevated, tc.severe), "OPCT-010A")
			assert.Equal(t, tc.want, res.Name, "actual=%s", res.Actual)
		})
	}
}

// OPCT-010A is advisory: it reports elevated latency but must never block.
func TestCheckOPCT010A_NeverFails(t *testing.T) {
	for _, elevated := range []int{0, 25, 50, 100} {
		res := runEtcdCheck(t, newEtcdSlowReport(100, elevated, elevated), "OPCT-010A")
		assert.NotEqual(t, CheckResultNameFail, res.Name, "elevated=%d", elevated)
	}
}

func TestCheckOPCT010B_SevereSlowRequests(t *testing.T) {
	for _, tc := range []struct {
		name          string
		total, severe int
		want          CheckResultName
	}{
		{"no severe events", 50, 0, CheckResultNamePass},
		{"at absolute pass ceiling", 50, 5, CheckResultNamePass},
		{"above count but within pass ratio", 100, 10, CheckResultNamePass},
		{"above pass bands", 50, 11, CheckResultNameWarn},
		{"at absolute warn ceiling", 50, 20, CheckResultNameWarn},
		{"above warn count but within warn ratio", 200, 40, CheckResultNameWarn},
		{"above warn count and ratio", 50, 21, CheckResultNameFail},
		{"sample below minimum", 9, 9, CheckResultNameSkip},
		// At the minimum sample size the absolute ceiling dominates: 10 severe
		// events is too few to block on, however bad the proportion looks.
		{"sample exactly at minimum", 10, 10, CheckResultNameWarn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := runEtcdCheck(t, newEtcdSlowReport(tc.total, 0, tc.severe), "OPCT-010B")
			assert.Equal(t, tc.want, res.Name, "actual=%s", res.Actual)
		})
	}
}

// The population is censored at etcd's own warning threshold, so improving a
// borderline-slow request removes it from the sample. Counting events per band
// must stay monotonic under that transformation: a cluster that got faster can
// never produce a worse verdict. This is the defect that the previous
// mean/maximum implementation exhibited.
func TestCheckOPCT010B_MonotonicUnderImprovement(t *testing.T) {
	severity := map[CheckResultName]int{
		CheckResultNamePass: 0,
		CheckResultNameWarn: 1,
		CheckResultNameFail: 2,
	}
	// 64 events, 13 severe: the published 4.23 AWS-CCM baseline.
	base := runEtcdCheck(t, newEtcdSlowReport(64, 39, 13), "OPCT-010B")

	t.Run("faster requests drop out of the logged population", func(t *testing.T) {
		// 13 sub-threshold events stop being logged; severe count is unchanged.
		res := runEtcdCheck(t, newEtcdSlowReport(51, 26, 13), "OPCT-010B")
		assert.LessOrEqual(t, severity[base.Name], severity[res.Name],
			"verdict must not improve spuriously")
	})

	t.Run("severe events becoming non-severe improves the verdict", func(t *testing.T) {
		res := runEtcdCheck(t, newEtcdSlowReport(64, 39, 3), "OPCT-010B")
		assert.GreaterOrEqual(t, severity[base.Name], severity[res.Name],
			"fewer severe events must never worsen the verdict")
	})
}

func TestCheckOPCT010_MissingData(t *testing.T) {
	stat := func(re *ReportData) *mustgather.BucketFilterStat {
		return re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"]
	}
	for _, tc := range []struct {
		name    string
		mutate  func(*ReportData)
		wantAct string
	}{
		{"no must-gather", func(re *ReportData) { re.Provider.MustGatherInfo = nil }, "ERR !must-gather"},
		{"no etcd logs", func(re *ReportData) { re.Provider.MustGatherInfo.ErrorEtcdLogs = nil }, "ERR !logs"},
		{"no parsed counters", func(re *ReportData) {
			re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll = nil
		}, "ERR !counters"},
		{"unparsable count", func(re *ReportData) { stat(re).StatCount = "" }, "ERR !count"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, id := range []string{"OPCT-010A", "OPCT-010B"} {
				re := newEtcdSlowReport(50, 10, 5)
				tc.mutate(re)
				res := runEtcdCheck(t, re, id)
				assert.Equal(t, tc.wantAct, res.Actual, "id=%s", id)
				assert.NotEqual(t, CheckResultNamePass, res.Name, "id=%s", id)
			}
		})
	}

	t.Run("missing severe bucket", func(t *testing.T) {
		re := newEtcdSlowReport(50, 10, 5)
		delete(stat(re).Buckets, mustgather.BucketRangeName1000Inf)
		res := runEtcdCheck(t, re, "OPCT-010B")
		assert.Equal(t, "ERR !bucket", res.Actual)
		assert.NotEqual(t, CheckResultNamePass, res.Name)
	})
}

// Regression guard over every archived opct-report-summary.json available when
// the bands were calibrated. The Red Hat reference CI baseline must not be
// blocked; only the materially degraded run may fail.
func TestCheckOPCT010B_ArchivedBaselines(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		total, elevated, severe int
		want                    CheckResultName
	}{
		{"4.22-aws", 49, 14, 5, CheckResultNamePass},
		{"4.22-aws-ccm", 38, 22, 8, CheckResultNameWarn},
		{"4.22-aws-gp3", 22, 8, 0, CheckResultNamePass},
		{"4.22-aws-ccm-gp3", 33, 15, 12, CheckResultNameWarn},
		{"4.23-aws", 22, 19, 10, CheckResultNameWarn},
		{"4.23-aws-b", 21, 16, 12, CheckResultNameWarn},
		{"4.23-aws-ccm", 27, 21, 12, CheckResultNameWarn},
		{"4.23-aws-ccm-b", 64, 39, 13, CheckResultNameWarn},
		{"5.0-aws", 14, 10, 1, CheckResultNamePass},
		{"5.0-aws-b", 35, 11, 3, CheckResultNamePass},
		{"5.0-aws-ccm", 37, 26, 15, CheckResultNameWarn},
		{"5.0-aws-ccm-b", 41, 20, 7, CheckResultNameWarn},
		{"5.1-aws", 52, 17, 12, CheckResultNameWarn},
		{"5.1-aws-b", 52, 30, 10, CheckResultNameWarn},
		{"5.1-aws-ccm", 58, 29, 19, CheckResultNameWarn},
		{"5.1-aws-ccm-b", 28, 20, 9, CheckResultNameWarn},
		// Materially degraded: 32 severe events, maximum ~4.99s.
		{"4.22-local-degraded", 101, 60, 32, CheckResultNameFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := runEtcdCheck(t, newEtcdSlowReport(tc.total, tc.elevated, tc.severe), "OPCT-010B")
			assert.Equal(t, tc.want, res.Name, "actual=%s", res.Actual)
		})
	}
}

func TestEtcdSlowCount(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{"13 (20.312%)", 13, false},
		{"0 (0.000%)", 0, false},
		{"64", 64, false},
		{"", 0, true},
		{"n/a", 0, true},
	} {
		t.Run(fmt.Sprintf("%q", tc.raw), func(t *testing.T) {
			got, err := etcdSlowCount(tc.raw)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
