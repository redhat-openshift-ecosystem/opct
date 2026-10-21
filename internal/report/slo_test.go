package report

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/archive"
	"github.com/redhat-openshift-ecosystem/opct/internal/openshift/mustgather"
)

func TestNewCheckSummary(t *testing.T) {
	checks := NewCheckSummary(&ReportData{})
	assert.NotNil(t, checks)

	// Check Names must not be higher than 88 characters
	for _, check := range checks.Checks {
		assert.Equal(t, true, len(check.ID) <= 9, "Check Name must not be higher than 8 characters: %s", check.ID)
		assert.Equal(t, true, len(check.Name) <= 88, "Check Name must not be higher than 88 characters: %s", check.Name)
	}

	// validate if checks IDs are in the format OPCT-NNN
	for _, check := range checks.Checks {
		if check.ID == CheckIdEmptyValue {
			continue
		}
		assert.Regexp(t, `OPCT-\d{3}`, check.ID, "Check ID must be in the format OPCT-NNN: %s", check.ID)
	}

	// DOC reference must exists in markdown file docs/review/rules.md
	for _, check := range checks.Checks {
		if check.ID == CheckIdEmptyValue {
			continue
		}
		assert.Contains(t, checks.generateDocumentation(), check.ID, "Check ID must be in the documentation: %s", check.ID)
	}
}

func TestGenerateDocumentation(t *testing.T) {
	checkSummary := &CheckSummary{
		Checks: []*Check{
			{
				ID:   "OPCT-001",
				Name: "Test Check 1",
				DocumentationSpec: CheckDocumentationSpec{
					Description:  "Description for Test Check 1",
					Action:       "Action for Test Check 1",
					Expected:     "Expected result for Test Check 1",
					Troubleshoot: "Troubleshoot for Test Check 1",
					Dependencies: []string{"OPCT-002"},
				},
			},
			{
				ID:   "OPCT-002",
				Name: "Test Check 2",
				DocumentationSpec: CheckDocumentationSpec{
					Description:  "Description for Test Check 2",
					Action:       "Action for Test Check 2",
					Expected:     "Expected result for Test Check 2",
					Troubleshoot: "Troubleshoot for Test Check 2",
				},
			},
			{
				ID:   CheckIdEmptyValue,
				Name: "Test Check 3",
				DocumentationSpec: CheckDocumentationSpec{
					Description:  "Description for Test Check 3",
					Action:       "Action for Test Check 3",
					Expected:     "Expected result for Test Check 3",
					Troubleshoot: "Troubleshoot for Test Check 3",
				},
			},
		},
		baseURL: "http://example.com/docs",
	}

	expectedDoc := `# OPCT Review/Check Rules

The OPCT rules are used in the report command to evaluate the data collected by the OPCT execution.
The HTML report will link directly to the rule ID on this page.

The rule details can be used as an additional resource in the review process.

The acceptance criteria for the rules are based on multiple CI jobs used as a reference to evaluate the expected result.
If you have any questions about the rules, please file an Issue in the OPCT repository.

## Rules
___

### OPCT-001

- **Name**: Test Check 1
- **Description**: Description for Test Check 1
- **Action**: Action for Test Check 1
- **Expected**:
Expected result for Test Check 1
- **Troubleshoot**:
Troubleshoot for Test Check 1
- **Dependencies**: [OPCT-002](#opct-002)

### OPCT-002

- **Name**: Test Check 2
- **Description**: Description for Test Check 2
- **Action**: Action for Test Check 2
- **Expected**:
Expected result for Test Check 2
- **Troubleshoot**:
Troubleshoot for Test Check 2

### Test Check 3

- **Name**: Test Check 3
- **Description**: Description for Test Check 3
- **Action**: Action for Test Check 3
- **Expected**:
Expected result for Test Check 3
- **Troubleshoot**:
Troubleshoot for Test Check 3

___
## Helper Rules Group

The following table describes how the check IDs are distributed.

| ID | Description |
| -- | --|
| 00X[\|A-Z] | Conformance result rules |
| 01X[\|A-Z] | Runtime, Infrastructure requirements, and known issues' rules |
| 02X[\|A-Z] | Result archive anomaly detector's rules |
| 03X[\|A-Z] | OpenShift object's rules |
| 04X[\|A-Z] | Red Hat OpenShift validation program's rules |


___
*<p style='text-align:center;'>Page generated automatically by <code>opct adm generate checks-docs</code></p>*`

	doc := checkSummary.generateDocumentation()
	assert.Equal(t, expectedDoc, doc)
}

func TestWriteDocumentation(t *testing.T) {
	t.Run("should_write_documentation_to_file", func(t *testing.T) {
		tmpDir := t.TempDir()
		docPath := filepath.Join(tmpDir, "test-doc-output.md")

		// Prepare a check summary
		csum := &CheckSummary{
			Checks: []*Check{
				{
					ID:   "OPCT-999",
					Name: "Fake Check",
					DocumentationSpec: CheckDocumentationSpec{
						Description:  "Fake check for testing WriteDocumentation",
						Action:       "No action",
						Expected:     "No expected result",
						Troubleshoot: "No troubleshoot info",
					},
				},
			},
			baseURL: "http://opct.ci/docs",
		}

		// Write doc
		err := csum.WriteDocumentation(docPath)
		assert.NoError(t, err)

		// Read file back
		data, readErr := os.ReadFile(docPath)
		assert.NoError(t, readErr)
		assert.Contains(t, string(data), "# OPCT Review/Check Rules", "Expected documentation header not found")
		assert.Contains(t, string(data), "OPCT-999", "Expected check ID to appear in documentation")
	})

	t.Run("should_handle_empty_checks_without_error", func(t *testing.T) {
		tmpDir := t.TempDir()
		docPath := filepath.Join(tmpDir, "empty-check-doc-output.md")

		csum := &CheckSummary{}
		err := csum.WriteDocumentation(docPath)
		assert.NoError(t, err)

		// Validate file exists
		_, statErr := os.Stat(docPath)
		assert.NoError(t, statErr)
	})
}

func TestCheckOPCT011ErrorBudget(t *testing.T) {
	runOPCT011 := func(ec *archive.ErrorCounter) CheckResult {
		re := &ReportData{Provider: &ReportResult{ErrorCounters: ec}}
		for _, check := range NewCheckSummary(re).Checks {
			if check.ID == "OPCT-011" {
				return check.Test()
			}
		}
		t.Fatal("check OPCT-011 not found")
		return CheckResult{}
	}
	counters := func(kv map[string]int) *archive.ErrorCounter {
		ec := archive.ErrorCounter(kv)
		return &ec
	}

	t.Run("a measured zero passes", func(t *testing.T) {
		res := runOPCT011(counters(map[string]int{"total": 0}))
		assert.Equal(t, CheckResultNamePass, res.Name,
			"a suite that ran without matching any error pattern must pass")
		assert.Equal(t, "0", res.Actual)
	})

	t.Run("an absent total fails", func(t *testing.T) {
		res := runOPCT011(counters(map[string]int{}))
		assert.Equal(t, CheckResultNameFail, res.Name,
			"unprocessed results must not be reported as zero errors")
		assert.Equal(t, "ERR !total", res.Actual)
	})

	t.Run("thresholds", func(t *testing.T) {
		for _, tc := range []struct {
			total int
			want  CheckResultName
		}{
			{total: 150, want: CheckResultNamePass},
			{total: 151, want: CheckResultNameWarn},
			{total: 300, want: CheckResultNameWarn},
			{total: 301, want: CheckResultNameFail},
		} {
			res := runOPCT011(counters(map[string]int{"total": tc.total}))
			assert.Equal(t, tc.want, res.Name, "total=%d", tc.total)
		}
	})
}

func etcdSlowReportData(statMean, statMax string) *ReportData {
	return &ReportData{
		Provider: &ReportResult{
			MustGatherInfo: &mustgather.MustGather{
				ErrorEtcdLogs: &mustgather.ErrorEtcdLogs{
					FilterRequestSlowAll: map[string]*mustgather.BucketFilterStat{
						"all": {StatMean: statMean, StatMax: statMax},
					},
				},
			},
		},
	}
}

func TestCheckOPCT010AAverageThresholds(t *testing.T) {
	runOPCT010A := func(statMean string) CheckResult {
		re := etcdSlowReportData(statMean, "0.000 (ms)")
		for _, check := range NewCheckSummary(re).Checks {
			if check.ID == "OPCT-010A" {
				return check.Test()
			}
		}
		t.Fatal("check OPCT-010A not found")
		return CheckResult{}
	}

	for _, tc := range []struct {
		mean float64
		want CheckResultName
	}{
		{mean: 499.999, want: CheckResultNamePass},
		{mean: 500.0, want: CheckResultNameWarn},
		{mean: 1000.0, want: CheckResultNameWarn},
		{mean: 1000.001, want: CheckResultNameFail},
	} {
		res := runOPCT010A(fmt.Sprintf("%.3f (ms)", tc.mean))
		assert.Equal(t, tc.want, res.Name, "mean=%v", tc.mean)
	}
}

func TestCheckOPCT010BMaximumThresholds(t *testing.T) {
	runOPCT010B := func(statMax string) CheckResult {
		re := etcdSlowReportData("0.000 (ms)", statMax)
		for _, check := range NewCheckSummary(re).Checks {
			if check.ID == "OPCT-010B" {
				return check.Test()
			}
		}
		t.Fatal("check OPCT-010B not found")
		return CheckResult{}
	}

	for _, tc := range []struct {
		max  float64
		want CheckResultName
	}{
		{max: 1499.999, want: CheckResultNamePass},
		{max: 1500.0, want: CheckResultNameWarn},
		{max: 2000.0, want: CheckResultNameWarn},
		{max: 2000.001, want: CheckResultNameFail},
	} {
		res := runOPCT010B(fmt.Sprintf("%.3f (ms)", tc.max))
		assert.Equal(t, tc.want, res.Name, "max=%v", tc.max)
	}
}
