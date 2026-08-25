package test

import (
	"embed"
	"io/fs"
	"testing"

	efs "github.com/redhat-openshift-ecosystem/opct/internal/assets"
	"github.com/redhat-openshift-ecosystem/opct/pkg/run"
	"github.com/stretchr/testify/assert"
)

//go:embed testdata
var testTemplatesPluginsAll embed.FS

func TestDataTemplatesPluginsManifests(t *testing.T) {
	type testCase struct {
		name   string
		assert func(tc *testCase)
	}
	cases := []testCase{
		{
			name: "process-manifest-template",
			assert: func(tc *testCase) {
				want, err := fs.ReadFile(efs.GetData(), "testdata/plugins/sample-v0-ok.yaml")
				if err != nil {
					t.Fatalf("failed to read plugin reference from efs: %v", err)
				}
				manifestTpl, err := fs.ReadFile(efs.GetData(), "testdata/templates/plugins/sample.yaml")
				if err != nil {
					t.Fatalf("failed to read plugin template from efs: %v", err)
				}
				options := &run.RunOptions{PluginsImage: "quay.io/opct/plugin:v0"}
				got, err := run.ProcessManifestTemplates(options, manifestTpl)
				if err != nil {
					t.Fatalf("failed to process plugin template: %v", err)
				}
				assert.Equal(t, string(want), string(got), "plugin manifests are readable")
			},
		},
		// TODO create tests for run.loadPluginManifests
	}

	efs.UpdateData(testTemplatesPluginsAll)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.assert(&tc)
		})
	}
}
