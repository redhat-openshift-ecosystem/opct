package archive

import (
	"regexp"
)

// CommonErrorPatterns is a list of common error patterns to be used to
// discover/calculate the error counter withing logs in archives (must-gather,
// conformance execution) by OPCT.
// Source: https://github.com/openshift/release/blob/master/core-services/prow/02_config/_config.yaml#L84
var CommonErrorPatterns = []string{
	// `error:`,
	`Failed to push image`,
	`Failed`,
	`timed out`,
	`'ERROR:'`,
	`ERRO\[`,
	`^error:`,
	`(^FAIL|FAIL: |Failure \[)\b`,
	`panic(\.go)?:`,
	`"level":"error"`,
	`level=error`,
	`level":"fatal"`,
	`level=fatal`,
	`│ Error:`,
	`client connection lost`,
}

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

	// check failures for each pattern
	for _, errName := range append(pattern, `error`) {
		reErr := regexp.MustCompile(errName)
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

// MergeErrorCounters is a method to merge two counter maps, resulting
// in a single containging all keys from both maps, and values accumulated
// by key.
func MergeErrorCounters(ec1, ec2 *ErrorCounter) *ErrorCounter {
	new := make(ErrorCounter, len(CommonErrorPatterns))
	if ec1 == nil {
		if ec2 == nil {
			return &new
		}
		return ec2
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
