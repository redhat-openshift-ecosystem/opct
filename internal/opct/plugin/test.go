package plugin

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/archive"
	"github.com/redhat-openshift-ecosystem/opct/internal/openshift/ci/sippy"
)

// TestItem represents a single test unit holding attributes for the processor
// pipeline.
type TestItem struct {
	// Name is the name of the e2e test. It is hidden from JSON as Tests is a map, and
	// the key can be used.
	Name string `json:"-"`

	// ID is the unique identifier of the test within the execution.
	ID string `json:"id"`

	// Status store the test result. Valid values: passed, skipped, failed.
	Status string `json:"status"`

	// State represents the state of the test. It can be any status value or filter name.
	State string `json:"state,omitempty"`

	// Failure contains the failure reason extracted from JUnit field 'item.detials.failure'.
	Failure string `json:"-"`

	// SystemOut contains the entire test stdout extracted from JUnit field 'item.detials.system-out'.
	SystemOut string `json:"-"`

	// Offset is the offset of failure from the plugin result file.
	Offset int `json:"-"`

	// Flaky contains the flake information from OpenShift CI - scraped from Sippy API.
	Flake *sippy.SippyTestsResponse `json:"flake,omitempty"`

	// ErrorCounters errors indexed by common error key.
	ErrorCounters archive.ErrorCounter `json:"errorCounters,omitempty"`

	// Reference for documentation.
	Documentation string `json:"documentation"`
}

type Tests map[string]*TestItem

// UpdateErrorCounter reads the failures and stdout looking for error patterns from
// a specific test, accumulating the ErrorCounters structure.
func (pi *TestItem) UpdateErrorCounter() {
	total := 0
	counters := make(archive.ErrorCounter, len(archive.CommonErrorPatterns)+1)

	incError := func(err string, cnt int) {
		if _, ok := counters[err]; !ok {
			counters[err] = 0
		}
		counters[err] += cnt
		total += cnt
	}

	for _, errName := range archive.CommonErrorPatterns {
		reErr := regexp.MustCompile(errName)
		// Check occurrences in Failure
		if matches := reErr.FindAllStringIndex(pi.Failure, -1); len(matches) != 0 {
			incError(errName, len(matches))
		}
		// Check occurrences in SystemOut
		if matches := reErr.FindAllStringIndex(pi.SystemOut, -1); len(matches) != 0 {
			incError(errName, len(matches))
		}
	}

	if total == 0 {
		return
	}
	pi.ErrorCounters = counters
	pi.ErrorCounters["total"] = total
}

// LookupDocumentation extracts from the test name the expected part (removing '[Conformance]')
// to link to the Documentation URL refereced by the Kubernetes Conformance markdown available
// at https://github.com/cncf/k8s-conformance/blob/master/docs/KubeConformance-<version>.md .
// The test documentation (TestDocumentation) should be indexed prior calling the LookupDocumentation.
func (pi *TestItem) LookupDocumentation(d *TestDocumentation) {

	// origin/openshift-tests appends 'labels' after '[Conformance]' in the
	// test name in the kubernetes/conformance, transforming it from the original name from upstream.
	// nameIndex will try to recover the original name to lookup in the source docs.
	nameIndex := fmt.Sprintf("%s[Conformance]", strings.Split(pi.Name, "[Conformance]")[0])

	// check if the test name is indexed in the conformance documentation.
	if _, ok := d.Tests[nameIndex]; ok {
		pi.Documentation = d.Tests[nameIndex].URLFragment
		return
	}
	// When the test is not indexed, no documentation will be added.
	pi.Documentation = *d.UserBaseURL
}
