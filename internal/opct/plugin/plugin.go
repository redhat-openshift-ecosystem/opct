package plugin

import (
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/archive"
)

const (
	PluginNameOpenShiftUpgrade      = "05-openshift-cluster-upgrade"
	PluginNameKubernetesConformance = "10-openshift-kube-conformance"
	PluginNameOpenShiftConformance  = "20-openshift-conformance-validated"
	PluginNameConformanceReplay     = "80-openshift-tests-replay"
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

	// DocumentationReference
	Documentation *TestDocumentation

	// Definition
	Definition *PluginDefinition

	// ErrorCounters is the map with details for each failure by regex expression.
	ErrorCounters archive.ErrorCounter `json:"errorCounters,omitempty"`

	// FailedItems is the map with details for each failure
	Tests Tests

	// FailedList is the list of tests failures on the original execution
	FailedList []string

	// FailedFiltered is the list of failures **after** filter(s) pipeline.
	// Those tests must raise attention and alerts.
	FailedFiltered []string

	// Filter SuiteOnly:
	// FailedFilter1 is the list of failures (A) included only in the original suite (B): A INTERSECTION B
	// FailedFilterSuite     []string
	FailedFilter1         []string
	FailedExcludedFilter1 []string

	// Filter Baseline:
	// FailedFilter2 is the list of failures (A) excluding the baseline(B): A EXCLUDE B
	// FailedFilterBaseline  []string
	FailedFilter2         []string
	FailedExcludedFilter2 []string

	// Filter FlakeAPI:
	// FailedFilter3 is the priority list of failures - not reporting as flake in OpenShift CI.
	// FailedFilterPrio      []string
	FailedFilter3         []string
	FailedExcludedFilter3 []string

	// Filter BaselineAPI:
	// FailedFilter4 is the list after excluding known failures from OPCT CI.
	// This filter is similar BaseLine, but it's a list of failures collected from
	// processed data (another OPCT execution) on OPCT CI after processed by OPCT report,
	// exposed thorugh the OPCT API. This list is used to exclude known failures,
	// to prevent false positives on the review pipeline.
	// TODO(mtulio): deprecate Filter2 when Filter4 is accurated. Baseline results should
	// not use Filter2.
	FailedFilter4         []string
	FailedExcludedFilter4 []string

	// Filter KnownFailures:
	// FailedFilter5 is the list of failures that are explicitly removed from pipeline.
	// It should not be used to exclude failures from the report of e2e included in suite,
	// but to remove known flake/failures that is not relevant to the pipeline.
	// Example: '[sig-arch] External binary usage'
	// Filter5KnownFailures  []string
	FailedFilter5         []string
	FailedExcludedFilter5 []string

	// Filter Replay:
	// FailedFilter6 is the list of failures which also failed in the second shot: replay plugin/step.
	FailedFilter6         []string
	FailedExcludedFilter6 []string
}

func (ps *OPCTPluginSummary) calculateErrorCounter() *archive.ErrorCounter {
	if ps.ErrorCounters == nil {
		ps.ErrorCounters = make(archive.ErrorCounter, len(archive.CommonErrorPatterns))
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

const (
	// FilterNameSuiteOnly is the filter to remove failures of tests not included in the suite.
	FilterNameSuiteOnly = "suite-only"

	// FilterNameKF is the filter to exclude known failures from the OPCT CI.
	FilterNameKF = "known-failures"

	// FilterNameBaseline is the filter to exclude failures from the baseline archive (CLI arg).
	FilterNameBaseline = "baseline"

	// FilterNameFlaky is the filter to exclude flaky tests from the report based in Sippy API.
	FilterNameFlaky = "flaky"

	// FilterNameReplay is the filter to exclude failures which are passing the replay step.
	FilterNameReplay = "replay"

	// FilterNameFinalCopy is the last step in the filter pipeline to copy the final list of failures
	// to be used to compose the final report/data.
	FilterNameFinalCopy = "copy"
)

// GetFailuresByFilterID returns the list of failures handlers by filter ID.
func (ps *OPCTPluginSummary) GetFailuresByFilterID(filterID string) ([]string, []string) {
	switch filterID {
	case FilterNameSuiteOnly:
		return ps.FailedFilter1, ps.FailedExcludedFilter1
	case FilterNameBaseline:
		return ps.FailedFilter2, ps.FailedExcludedFilter2
	case FilterNameKF:
		return ps.FailedFilter5, ps.FailedExcludedFilter5
	case FilterNameReplay:
		return ps.FailedFilter6, ps.FailedExcludedFilter6
	}
	return nil, nil
}

// SetFailuresByFilterID stes the list of failures handlers by filter ID.
func (ps *OPCTPluginSummary) SetFailuresByFilterID(filterID string, failures []string, excluded []string) {
	switch filterID {
	case FilterNameSuiteOnly:
		ps.FailedFilter1 = failures
		ps.FailedExcludedFilter1 = excluded
		return
	case FilterNameBaseline:
		ps.FailedFilter2 = failures
		ps.FailedExcludedFilter2 = excluded
		return
	case FilterNameKF:
		ps.FailedFilter5 = failures
		ps.FailedExcludedFilter5 = excluded
		return
	case FilterNameReplay:
		ps.FailedFilter6 = failures
		ps.FailedExcludedFilter6 = excluded
		return
	}
}

// GetPreviousFailuresByFilterID returns the list of failures from the previous plugin
// in the pipeline, by providing the current filter ID.
// TODO: move the filter logic to a dedicated structure using linked stack/list,
// allowing each plugin having a dynamic list of filters, instead of forcing the same
// pipeline across all plugins.
func (ps *OPCTPluginSummary) GetPreviousFailuresByFilterID(filterID string) []string {
	switch filterID {
	case FilterNameSuiteOnly:
		return nil
	case FilterNameKF:
		return ps.FailedFilter1 // SuiteOnly
	case FilterNameReplay:
		return ps.FailedFilter5 // KnownFailures
	case FilterNameBaseline:
		return ps.FailedFilter6 // Replay
	case FilterNameFinalCopy:
		return ps.FailedFilter4 // BaselineAPI
	}
	return nil
}
