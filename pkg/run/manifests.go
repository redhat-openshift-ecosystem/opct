package run

import (
	"bytes"
	"fmt"
	"io/fs"
	"text/template"

	efs "github.com/redhat-openshift-ecosystem/opct/internal/assets"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	log "github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/loader"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/manifest"
)

// ProcessManifestTemplates processes go template variables in the manifest which map to variable in RunOptions
func ProcessManifestTemplates(r *RunOptions, manifest []byte) ([]byte, error) {
	pluginTpl, err := template.New("manifest").Parse(string(manifest))
	if err != nil {
		return nil, fmt.Errorf("unable to parse manifest: %w", err)
	}
	var imageBuffer bytes.Buffer
	err = pluginTpl.Execute(&imageBuffer, r)
	if err != nil {
		return nil, fmt.Errorf("unable to update manifest: %w", err)
	}
	return imageBuffer.Bytes(), nil
}

// loadPluginManifests reads the plugin manifests from embed FS, render the
// template and creates the sonobuoy's manifest slice.
func loadPluginManifests(r *RunOptions) ([]*manifest.Manifest, error) {
	var manifests []*manifest.Manifest

	pluginManifests, err := efs.GetAllFilenames(efs.GetData(), "data/templates/plugins")
	if err != nil {
		log.Error("Unable to load plugin manifest files.")
		return nil, err
	}
	for _, m := range pluginManifests {
		log.Debugf("Loading plugin: %s", m)
		pluginManifestTpl, err := fs.ReadFile(efs.GetData(), m)
		if err != nil {
			log.Errorf("error reading config for plugin %s: %v", m, err)
			return nil, err
		}
		pluginManifest, err := ProcessManifestTemplates(r, pluginManifestTpl)
		if err != nil {
			log.Errorf("error processing configuration for plugin %s: %v", m, err)
			return nil, err
		}

		// Print rendered manifest if flag is enabled
		if r.verbose {
			fmt.Printf("\n---\n# Rendered manifest for: %s\n---\n%s\n", m, string(pluginManifest))
		}

		asset, err := loader.LoadDefinition(pluginManifest)
		if err != nil {
			log.Errorf("error loading configuration for plugin %s: %v", m, err)
			return nil, err
		}

		// Skip conformance plugins (10, 20, 80) in upgrade mode.
		// These plugins produce invalid results due to binary/release version
		// mismatch when the cluster is upgraded mid-run.
		if r.mode == plugin.WorkflowUpgrade {
			pluginName := asset.SonobuoyConfig.PluginName
			switch pluginName {
			case plugin.PluginNameKubernetesConformance,
				plugin.PluginNameOpenShiftConformance,
				plugin.PluginNameConformanceReplay:
				log.Infof("Skipping plugin %s in upgrade mode", pluginName)
				continue
			}
		}

		manifests = append(manifests, &asset)
	}

	return manifests, nil
}
