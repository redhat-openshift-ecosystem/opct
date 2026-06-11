package run

import "testing"

// TestResolveKubernetesSuiteName verifies the version-based suite selection for OCP 4.x and 5.x clusters.
func TestResolveKubernetesSuiteName(t *testing.T) {
	tests := []struct {
		name     string
		major    int
		minor    int
		expected string
	}{
		{
			name:     "OCP 4.19 uses serial suite",
			major:    4,
			minor:    19,
			expected: "kubernetes/conformance",
		},
		{
			name:     "OCP 4.20 uses parallel suite",
			major:    4,
			minor:    20,
			expected: "kubernetes/conformance/parallel",
		},
		{
			name:     "OCP 4.21 uses parallel suite",
			major:    4,
			minor:    21,
			expected: "kubernetes/conformance/parallel",
		},
		{
			name:     "OCP 4.18 uses serial suite",
			major:    4,
			minor:    18,
			expected: "kubernetes/conformance",
		},
		{
			name:     "OCP 5.0 uses parallel suite",
			major:    5,
			minor:    0,
			expected: "kubernetes/conformance/parallel",
		},
		{
			name:     "OCP 5.1 uses parallel suite",
			major:    5,
			minor:    1,
			expected: "kubernetes/conformance/parallel",
		},
		{
			name:     "OCP 4.0 uses serial suite",
			major:    4,
			minor:    0,
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
