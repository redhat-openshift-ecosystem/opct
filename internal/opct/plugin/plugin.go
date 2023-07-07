package plugin

import (
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/archive"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/openshift/ci"
)

const (
	PluginNameOpenShiftUpgrade      = "05-openshift-cluster-upgrade"
	PluginNameKubernetesConformance = "10-openshift-kube-conformance"
	PluginNameOpenShiftConformance  = "20-openshift-conformance-validated"
	PluginNameArtifactsCollector    = "99-openshift-artifacts-collector"

	// Old Plugin names (prior v0.2). It's used to keep compatibility
	PluginOldNameKubernetesConformance = "openshift-kube-conformance"
	PluginOldNameOpenShiftConformance  = "openshift-conformance-validated"
)

type PluginDefinition struct {
	PluginImage   string `json:"pluginImage"`
	SonobuoyImage string `json:"sonobuoyImage"`
	Name          string `json:"name"`
}

// OPCTPluginSummary handle plugin details
type OPCTPluginSummary struct {
	Name      string
	NameAlias string
	Status    string
	Total     int64
	Passed    int64
	Failed    int64
	Timeout   int64
	Skipped   int64

	// FailedItems is the map with details for each failure
	Tests Tests

	// FailedList is the list of tests failures on the original execution
	FailedList []string

	// FailedFilterSuite is the list of failures (A) included only in the original suite (B): A INTERSECTION B
	FailedFilterSuite []string

	// FailedFilterBaseline is the list of failures (A) excluding the baseline(B): A EXCLUDE B
	FailedFilterBaseline []string

	// FailedFilterPrio is the priority list of failures - not reporting as flake in OpenShift CI.
	FailedFilterPrio []string

	// DocumentationReference
	Documentation *TestDocumentation

	// Definition
	Definition *PluginDefinition

	ErrorCounters archive.ErrorCounter `json:"errorCounters,omitempty"`
}

func (ps *OPCTPluginSummary) calculateErrorCounter() *archive.ErrorCounter {
	if ps.ErrorCounters == nil {
		ps.ErrorCounters = make(archive.ErrorCounter, len(ci.CommonErrorPatterns))
	}
	for _, test := range ps.Tests {
		if test.ErrorCounters == nil {
			continue
		}
		for kerr, errName := range test.ErrorCounters {
			if _, ok := ps.ErrorCounters[kerr]; !ok {
				ps.ErrorCounters[kerr] = errName
			} else {
				ps.ErrorCounters[kerr] += errName
			}
		}
	}
	return &ps.ErrorCounters
}

func (ps *OPCTPluginSummary) GetErrorCounters() *archive.ErrorCounter {
	return ps.calculateErrorCounter()
}
