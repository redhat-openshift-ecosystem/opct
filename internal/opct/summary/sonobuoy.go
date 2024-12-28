package summary

import (
	"bytes"
	"strings"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/archive"
	"github.com/vmware-tanzu/sonobuoy/pkg/discovery"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/manifest"
	v1 "k8s.io/api/core/v1"
)

type SonobuoyPluginDefinitionManifest = manifest.Manifest

// Plugin is the sonobuoy plugin definitoin.
type SonobuoyPluginDefinition struct {
	Definition    *SonobuoyPluginDefinitionManifest `json:"Definition"`
	SonobuoyImage string                            `json:"SonobuoyImage"`
}

type SonobuoySummary struct {
	Cluster           *discovery.ClusterSummary
	MetaRuntime       []*archive.RuntimeInfoItem
	MetaConfig        []*archive.RuntimeInfoItem
	OpctConfig        []*archive.RuntimeInfoItem
	PluginsDefinition map[string]*SonobuoyPluginDefinition
}

func NewSonobuoySummary() *SonobuoySummary {
	return &SonobuoySummary{
		PluginsDefinition: make(map[string]*SonobuoyPluginDefinition, 5),
	}
}

func (s *SonobuoySummary) SetCluster(c *discovery.ClusterSummary) error {
	s.Cluster = c
	return nil
}

func (s *SonobuoySummary) SetPluginsDefinition(p map[string]*SonobuoyPluginDefinition) error {
	s.PluginsDefinition = make(map[string]*SonobuoyPluginDefinition, len(p))
	s.PluginsDefinition = p
	return nil
}

func (s *SonobuoySummary) SetPluginDefinition(name string, def *SonobuoyPluginDefinition) {
	s.PluginsDefinition[name] = def
}

func (s *SonobuoySummary) ParseMetaRunlogs(logLines *bytes.Buffer) {
	s.MetaRuntime = archive.ParseMetaLogs(strings.Split(logLines.String(), "\n"))
}

func (s *SonobuoySummary) ParseMetaConfig(metaConfig *archive.MetaConfigSonobuoy) {
	s.MetaConfig = archive.ParseMetaConfig(metaConfig)
}

func (s *SonobuoySummary) ParseOpctConfigMap(cm *v1.ConfigMapList) {
	s.OpctConfig = archive.ParseOpctConfig(cm)
}
