package cleaner

import (
	"bytes"
	"strings"
)

const (
	maxLeakScanSize    = 10 * 1024 * 1024 // 10MB
	redactionMarker    = "<REDACTED_BY_OPCT>"
	redactionMarkerLen = len(redactionMarker)
)

// ScanContentForLeaks scans file content against the embedded leak patterns.
// Returns a list of findings. Skips binary files and files exceeding size limit.
func ScanContentForLeaks(filename string, content []byte) []LeakFinding {
	if len(content) == 0 || len(content) > maxLeakScanSize {
		return nil
	}

	if isBinary(content) {
		return nil
	}

	contentLower := bytes.ToLower(content)
	lines := bytes.Split(content, []byte("\n"))
	var findings []LeakFinding

	for i := range leakPatterns {
		p := &leakPatterns[i]

		if !keywordMatch(contentLower, p.Keywords) {
			continue
		}

		for lineNum, line := range lines {
			if p.Regex.Match(line) {
				findings = append(findings, LeakFinding{
					File:    filename,
					Pattern: p.Description,
					Line:    lineNum + 1,
				})
				break
			}
		}

		if len(findings) > 0 && findings[len(findings)-1].Pattern == p.Description {
			continue
		}

		if p.Regex.Match(content) {
			findings = append(findings, LeakFinding{
				File:    filename,
				Pattern: p.Description,
				Line:    0,
			})
		}
	}

	return findings
}

// ScanAndRedactLeaks scans file content for leaks and redacts detected patterns.
// Returns redacted content and list of findings.
// Skips binary files and files exceeding size limit.
func ScanAndRedactLeaks(filename string, content []byte) ([]byte, []LeakFinding) {
	if len(content) == 0 || len(content) > maxLeakScanSize {
		return content, nil
	}

	if isBinary(content) {
		return content, nil
	}

	contentLower := bytes.ToLower(content)
	var findings []LeakFinding
	redacted := content

	for i := range leakPatterns {
		p := &leakPatterns[i]

		if !keywordMatch(contentLower, p.Keywords) {
			continue
		}

		// Find all matches with submatches (for patterns with capture groups)
		matches := p.Regex.FindAllSubmatchIndex(redacted, -1)
		if matches == nil {
			continue
		}

		// Record findings (before redaction for line numbers)
		lines := bytes.Split(content, []byte("\n"))
		for lineNum, line := range lines {
			if p.Regex.Match(line) {
				findings = append(findings, LeakFinding{
					File:    filename,
					Pattern: p.Description,
					Line:    lineNum + 1,
				})
				break // Only record first occurrence per pattern per file
			}
		}

		// If no line match found but pattern matches overall
		if len(findings) == 0 || findings[len(findings)-1].Pattern != p.Description {
			findings = append(findings, LeakFinding{
				File:    filename,
				Pattern: p.Description,
				Line:    0,
			})
		}

		// Redact all matches (iterate backwards to preserve indices)
		// If pattern has capture groups, redact only the first capture group
		// Otherwise redact the entire match
		marker := []byte(redactionMarker)
		for j := len(matches) - 1; j >= 0; j-- {
			match := matches[j]

			// Determine what to redact:
			// - If there are capture groups (len > 2), use the first capture group
			// - Otherwise use the full match
			var start, end int
			if len(match) > 2 && match[2] != -1 {
				// First capture group indices are at [2] and [3]
				start = match[2]
				end = match[3]
			} else {
				// Full match indices are at [0] and [1]
				start = match[0]
				end = match[1]
			}

			// Preserve trailing newline/whitespace if present
			trailing := []byte{}
			if end > start && (redacted[end-1] == '\n' || redacted[end-1] == '\r') {
				trailing = redacted[end-1 : end]
				end--
			}

			// Replace secret with redaction marker + preserved trailing chars
			redacted = append(redacted[:start], append(marker, append(trailing, redacted[end:]...)...)...)
		}
	}

	return redacted, findings
}

func isBinary(content []byte) bool {
	checkLen := min(512, len(content))
	return bytes.ContainsRune(content[:checkLen], 0)
}

func keywordMatch(contentLower []byte, keywords []string) bool {
	if len(keywords) == 0 {
		return true
	}
	for _, kw := range keywords {
		if bytes.Contains(contentLower, []byte(strings.ToLower(kw))) {
			return true
		}
	}
	return false
}
