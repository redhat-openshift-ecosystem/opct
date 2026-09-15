package run

import "testing"

func TestUnsupportedSkipChecksAlias(t *testing.T) {
	cmd := NewCmdRun()
	if cmd.Flags().Lookup("unsupported-skip-checks") == nil {
		t.Fatal("unsupported-skip-checks flag is not registered")
	}
	if err := cmd.Flags().Set("unsupported-skip-checks", "true"); err != nil {
		t.Fatalf("setting unsupported-skip-checks: %v", err)
	}

	got, err := cmd.Flags().GetBool("devel-skip-checks")
	if err != nil {
		t.Fatalf("getting devel-skip-checks: %v", err)
	}
	if !got {
		t.Error("unsupported-skip-checks did not set devel-skip-checks")
	}
}

// TestResolveKubernetesSuiteName verifies version-based suite selection.
func TestResolveKubernetesSuiteName(t *testing.T) {
	tests := []struct {
		name     string
		major    int
		minor    int
		expected string
	}{
		{
			name:     "OCP 4.0 uses conformance suite",
			major:    4,
			minor:    0,
			expected: "kubernetes/conformance",
		},
		{
			name:     "OCP 4.18 uses conformance suite",
			major:    4,
			minor:    18,
			expected: "kubernetes/conformance",
		},
		{
			name:     "OCP 4.19 uses conformance suite",
			major:    4,
			minor:    19,
			expected: "kubernetes/conformance",
		},
		{
			name:     "OCP 4.20 uses conformance suite",
			major:    4,
			minor:    20,
			expected: "kubernetes/conformance",
		},
		{
			name:     "OCP 4.21 uses conformance suite",
			major:    4,
			minor:    21,
			expected: "kubernetes/conformance",
		},
		{
			name:     "OCP 4.22 uses conformance suite",
			major:    4,
			minor:    22,
			expected: "kubernetes/conformance",
		},
		{
			name:     "OCP 5.0 uses conformance suite",
			major:    5,
			minor:    0,
			expected: "kubernetes/conformance",
		},
		{
			name:     "OCP 5.1 uses conformance suite",
			major:    5,
			minor:    1,
			expected: "kubernetes/conformance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveKubernetesSuiteName(tt.major, tt.minor)
			if got != tt.expected {
				t.Errorf("resolveKubernetesSuiteName(%d, %d) = %q, want %q",
					tt.major, tt.minor, got, tt.expected)
			}
		})
	}
}
