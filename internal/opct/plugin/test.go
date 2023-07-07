package plugin

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/archive"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/openshift/ci"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/openshift/ci/sippy"
)

type Tests map[string]*TestItem

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

func (pi *TestItem) UpdateErrorCounter() {
	total := 0
	counters := make(archive.ErrorCounter, len(ci.CommonErrorPatterns)+1)

	incError := func(err string, cnt int) {
		if _, ok := counters[err]; !ok {
			counters[err] = 0
		}
		counters[err] += cnt
		total += cnt
	}

	for _, errName := range ci.CommonErrorPatterns {
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

func (pi *TestItem) LookupDocumentation(d *TestDocumentation) {

	// origin/openshift-tests appends 'labels' after '[Conformance]'in the
	// plugin name, transforming it from the original name from upstream.
	// nameIndex will try to recover the original name to lookup in the source docs.
	nameIndex := fmt.Sprintf("%s[Conformance]", strings.Split(pi.Name, "[Conformance]")[0])
	if _, ok := d.Tests[nameIndex]; ok {
		pi.Documentation = d.Tests[nameIndex].PageLink
		return
	}
	// When the test is not indexed, no documentation will be added.
	// pi.DocsReference = *d.UserBaseURL
}
