package run

import (
	"testing"
)

// TestCheckPluginImages tests the checkPluginImages function
// Note: This function executes 'oc image info' which requires the oc binary
// to be available in the PATH. Full testing would be done in integration tests.
func TestCheckPluginImages(t *testing.T) {
	tests := []struct {
		name          string
		images        []string
		expectErrors  bool
		errorCount    int
		skipExecution bool // Skip actual execution for unit tests
	}{
		{
			name:          "empty image list",
			images:        []string{},
			expectErrors:  false,
			errorCount:    0,
			skipExecution: false,
		},
		{
			name:          "image list with empty strings",
			images:        []string{"", "", ""},
			expectErrors:  false,
			errorCount:    0,
			skipExecution: false,
		},
		{
			name:          "valid image format",
			images:        []string{"quay.io/opct/tools:latest"},
			expectErrors:  false,
			skipExecution: true, // Would need oc binary and network access
		},
		{
			name:          "multiple images with empty strings",
			images:        []string{"", "quay.io/opct/tools:latest", "", "quay.io/opct/plugins:latest"},
			expectErrors:  false,
			skipExecution: true, // Would need oc binary and network access
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipExecution {
				t.Skip("Skipping test that requires oc binary and network access")
				return
			}

			errs := checkPluginImages(tt.images)

			if tt.expectErrors && len(errs) == 0 {
				t.Errorf("checkPluginImages() expected errors but got none")
			}

			if !tt.expectErrors && len(errs) > 0 {
				t.Errorf("checkPluginImages() unexpected errors: %v", errs)
			}

			if tt.errorCount > 0 && len(errs) != tt.errorCount {
				t.Errorf("checkPluginImages() expected %d errors, got %d", tt.errorCount, len(errs))
			}
		})
	}
}

// TestCheckPluginImages_EmptyStrings verifies that empty image strings are skipped
func TestCheckPluginImages_EmptyStrings(t *testing.T) {
	images := []string{"", "", ""}
	errs := checkPluginImages(images)

	if len(errs) > 0 {
		t.Errorf("checkPluginImages() should skip empty strings without errors, got: %v", errs)
	}
}

// TestRunOptions_DryRunFlag tests that the dryRun flag is properly initialized
func TestRunOptions_DryRunFlag(t *testing.T) {
	opts := newRunOptions()

	// Verify dryRun is initialized to false (default)
	if opts.dryRun != false {
		t.Errorf("RunOptions.dryRun should default to false, got: %v", opts.dryRun)
	}
}
