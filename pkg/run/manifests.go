package run

import (
	"bytes"
	"text/template"

	"github.com/pkg/errors"
	efs "github.com/redhat-openshift-ecosystem/opct/internal/assets"
	log "github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/loader"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/manifest"
)

// ProcessManifestTemplates processes go template variables in the manifest which map to variable in RunOptions
func ProcessManifestTemplates(r *RunOptions, manifest []byte) ([]byte, error) {
	pluginTpl, err := template.New("manifest").Parse(string(manifest))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse manifest ")
	}
	var imageBuffer bytes.Buffer
	err = pluginTpl.Execute(&imageBuffer, r)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to update manifest")
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
		pluginManifestTpl, err := efs.GetData().ReadFile(m)
		if err != nil {
			log.Errorf("error reading config for plugin %s: %v", m, err)
			return nil, err
		}
		pluginManifest, err := ProcessManifestTemplates(r, pluginManifestTpl)
		if err != nil {
			log.Errorf("error processing configuration for plugin %s: %v", m, err)
			return nil, err
		}
		asset, err := loader.LoadDefinition(pluginManifest)
		if err != nil {
			log.Errorf("error loading configuration for plugin %s: %v", m, err)
			return nil, err
		}
		manifests = append(manifests, &asset)
	}

	return manifests, nil
}
