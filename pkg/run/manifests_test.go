package run

import (
	"embed"
	"testing"

	efs "github.com/redhat-openshift-ecosystem/opct/internal/assets"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed data/templates
var testPluginTemplates embed.FS

// TestLoadPluginManifests validates that loadPluginManifests returns the correct
// set of plugin manifests depending on the run mode (upgrade vs default).
func TestLoadPluginManifests(t *testing.T) {
	// Initialize the embed FS with the test data matching the production
	// path layout (data/templates/plugins/*.yaml).
	originalFS := efs.GetData()
	efs.UpdateData(&testPluginTemplates)
	t.Cleanup(func() {
		if originalFS != nil {
			efs.UpdateData(originalFS)
		}
	})

	// RunOptions must have image fields populated so that
	// ProcessManifestTemplates can render the Go templates
	// ({{ .PluginsImage }}, {{ .CollectorImage }}, etc.).
	baseOpts := func() *RunOptions {
		o := newRunOptions()
		o.PluginsImage = "quay.io/opct/plugin:test"
		o.OpenshiftTestsImage = "image-registry.openshift-image-registry.svc:5000/openshift/tests"
		o.CollectorImage = "quay.io/opct/collector:test"
		o.MustGatherMonitoringImage = "quay.io/opct/must-gather:test"
		return o
	}

	t.Run("upgrade mode skips conformance plugins", func(t *testing.T) {
		opts := baseOpts()
		opts.mode = plugin.WorkflowUpgrade

		manifests, err := loadPluginManifests(opts)
		require.NoError(t, err)

		// Collect returned plugin names.
		var names []string
		for _, m := range manifests {
			names = append(names, m.SonobuoyConfig.PluginName)
		}

		// In upgrade mode, only the upgrade and collector plugins should be loaded.
		assert.Contains(t, names, plugin.PluginNameOpenShiftUpgrade,
			"upgrade plugin must be present in upgrade mode")
		assert.Contains(t, names, plugin.PluginNameArtifactsCollector,
			"artifacts-collector plugin must be present in upgrade mode")

		// Conformance plugins must NOT be present.
		assert.NotContains(t, names, plugin.PluginNameKubernetesConformance,
			"kube-conformance plugin must be skipped in upgrade mode")
		assert.NotContains(t, names, plugin.PluginNameOpenShiftConformance,
			"openshift-conformance plugin must be skipped in upgrade mode")
		assert.NotContains(t, names, plugin.PluginNameConformanceReplay,
			"conformance-replay plugin must be skipped in upgrade mode")

		// Exactly 2 plugins expected.
		assert.Len(t, manifests, 2,
			"upgrade mode should load exactly 2 plugins (upgrade + collector)")

		// PLUGIN_BLOCKED_BY must be empty for the collector in upgrade mode
		// since the replay plugin (80) is skipped and would never complete.
		for _, m := range manifests {
			if m.SonobuoyConfig.PluginName == plugin.PluginNameArtifactsCollector {
				for _, env := range m.Spec.Env {
					if env.Name == "PLUGIN_BLOCKED_BY" {
						assert.Empty(t, env.Value,
							"PLUGIN_BLOCKED_BY must be empty in upgrade mode")
					}
				}
			}
		}
	})

	t.Run("default mode loads all plugins", func(t *testing.T) {
		opts := baseOpts()
		// mode is empty (default, non-upgrade)

		manifests, err := loadPluginManifests(opts)
		require.NoError(t, err)

		var names []string
		for _, m := range manifests {
			names = append(names, m.SonobuoyConfig.PluginName)
		}

		// All 5 plugins should be loaded in the default mode.
		assert.Contains(t, names, plugin.PluginNameOpenShiftUpgrade)
		assert.Contains(t, names, plugin.PluginNameKubernetesConformance)
		assert.Contains(t, names, plugin.PluginNameOpenShiftConformance)
		assert.Contains(t, names, plugin.PluginNameConformanceReplay)
		assert.Contains(t, names, plugin.PluginNameArtifactsCollector)

		assert.Len(t, manifests, 5,
			"default mode should load all 5 plugins")
	})
}
