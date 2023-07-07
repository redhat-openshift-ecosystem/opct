package archive

import (
	"regexp"

	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/openshift/ci"
)

// ErrorCounter is a map to handle a generic error counter, indexed by error pattern.
type ErrorCounter map[string]int

func NewErrorCounter(buf *string, pattern []string) ErrorCounter {
	total := 0
	counters := make(ErrorCounter, len(pattern)+2)

	incError := func(err string, cnt int) {
		if _, ok := counters[err]; !ok {
			counters[err] = 0
		}
		counters[err] += cnt
		total += cnt
	}

	for _, errName := range append(pattern, `error`) {
		reErr := regexp.MustCompile(errName)
		// Check occurrences in Failure
		if matches := reErr.FindAllStringIndex(*buf, -1); len(matches) != 0 {
			incError(errName, len(matches))
		}
	}

	if total == 0 {
		return nil
	}
	counters["total"] = total
	return counters
}

func MergeErrorCounters(ec1, ec2 *ErrorCounter) *ErrorCounter {
	new := make(ErrorCounter, len(ci.CommonErrorPatterns))
	if ec1 == nil {
		return &new
	}
	if ec2 == nil {
		return ec1
	}
	for kerr, errName := range *ec1 {
		if _, ok := new[kerr]; !ok {
			new[kerr] = errName
		} else {
			new[kerr] += errName
		}
	}
	for kerr, errName := range *ec2 {
		if _, ok := new[kerr]; !ok {
			new[kerr] = errName
		} else {
			new[kerr] += errName
		}
	}
	return &new
}
