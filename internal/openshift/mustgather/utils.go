package mustgather

import (
	"archive/tar"
	"bytes"
	"regexp"

	"github.com/ulikunitz/xz"
)

const (
	// patterns to match files in must-gather to be collected/processed.
	// patternNamePodLogs represents the pattern to match pod logs.
	patternNamePodLogs string = "logs"
	patternFilePodLogs string = `(\/namespaces\/.*\/pods\/.*.log)`

	// patternNameEvents represents the pattern to match the event filter file.
	patternNameEvents string = "events"
	patternFileEvents string = `(\/event-filter.html)`

	// patternNameRawFile represents the pattern to match raw files (any desired to collect).
	patternNameRawFile string = "rawFile"
	patternFileRawFile string = `(\/etcd_info\/.*.json)`

	// patternNamePodNetCheck represents the pattern to match pod network check files.
	patternNamePodNetCheck string = "podNetCheck"
	patternFilePodNetCheck string = `(\/pod_network_connectivity_check\/podnetworkconnectivitychecks.yaml)`
)

var (
	mustGatherFilePatterns = map[string]string{
		patternNamePodLogs:     `(\/namespaces\/.*\/pods\/.*.log)`,
		patternNameEvents:      `(\/event-filter.html)`,
		patternNameRawFile:     `(\/etcd_info\/.*.json)`,
		patternNamePodNetCheck: `(\/pod_network_connectivity_check\/podnetworkconnectivitychecks.yaml)`,
	}
)

// getFileTypeToProcess define patterns to continue the must-gather processor.
// the pattern must be defined if the must be extracted. It will return
// a boolean with match and the file group (pattern type).
func getFileTypeToProcess(path string) (bool, string) {
	for typ, pattern := range mustGatherFilePatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(path) {
			return true, typ
		}
	}
	return false, ""
}

// normalizeRelativePath removes the prefix of must-gather path/image to save the
// relative file path when extracting the file or mapping in the counters.
// OPCT collects must-gather automatically saving in the directory must-gather-opct.
func normalizeRelativePath(file string) string {
	re := regexp.MustCompile(`must-gather-opct/([A-Za-z0-9]+(-[A-Za-z0-9]+)+\/)`)

	split := re.Split(file, -1)
	if len(split) != 2 {
		return file
	}
	return split[1]
}

func getTarFromXZBuffer(buf *bytes.Buffer) (*tar.Reader, error) {
	file, err := xz.NewReader(buf)
	if err != nil {
		return nil, err
	}
	return tar.NewReader(file), nil
}
