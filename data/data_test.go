package data

import (
	"embed"
	"testing"

	efs "github.com/redhat-openshift-ecosystem/opct/internal/assets"
	"github.com/stretchr/testify/assert"
)

//go:embed templates/plugins
var testTemplatesPluginsAll embed.FS

// TestDataTemplatesPluginsManifests asserts required plugin manifests are present in EFS.
func TestDataTemplatesPluginsManifests(t *testing.T) {
	type testCase struct {
		name   string
		assert func(tc *testCase)
	}
	cases := []testCase{
		{
			name: "plugins-manifest-required",
			assert: func(tc *testCase) {
				want := []string{
					"templates/plugins/openshift-artifacts-collector.yaml",
					"templates/plugins/openshift-cluster-upgrade.yaml",
					"templates/plugins/openshift-conformance-replay.yaml",
					"templates/plugins/openshift-conformance-validated.yaml",
					"templates/plugins/openshift-kube-conformance.yaml",
				}
				got, err := efs.GetAllFilenames(efs.GetData(), "templates/plugins")
				if err != nil {
					t.Fatalf("failed to read efs: %v", err)
				}
				assert.Equal(t, want, got, "plugin manifest files are present")
			},
		},
		{
			name: "plugins-manifest-readable",
			assert: func(tc *testCase) {
				want := true
				got := false
				manifests, err := efs.GetAllFilenames(efs.GetData(), "templates/plugins")
				if err != nil {
					t.Fatalf("failed to read efs: %v", err)
				}
				for _, m := range manifests {
					manifestFile, err := efs.GetData().ReadFile(m)
					if err != nil {
						t.Fatalf("unable to read manifest %s: %v", m, err)
					}
					if len(manifestFile) == 0 {
						t.Fatalf("empty manifest %s", m)
					}
				}
				got = true
				assert.Equal(t, want, got, "plugin manifests are readable")
			},
		},
		// TODO: add test for plugin manifest content are rendered as expected.
	}

	efs.UpdateData(&testTemplatesPluginsAll)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.assert(&tc)
		})
	}
}
