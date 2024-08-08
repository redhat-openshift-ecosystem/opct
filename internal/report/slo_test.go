package report

// TODO(mtulio): create unit:
// - name should not have more than X size
// - ID must be in the format OPCT-NNN
// - DOC reference must exists in docs/review/rules.md
// - returns should be pass or fail

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCheckSummary(t *testing.T) {
	checks := NewCheckSummary(&ReportData{})
	assert.NotNil(t, checks)

	// Check Names must not be higher than 88 characters
	for _, check := range checks.Checks {
		assert.Equal(t, true, len(check.ID) <= 9, "Check Name must not be higher than 8 characters: %s", check.ID)
		assert.Equal(t, true, len(check.Name) <= 88, "Check Name must not be higher than 88 characters: %s", check.Name)
	}
}
