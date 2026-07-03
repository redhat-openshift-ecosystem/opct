package cleaner

import (
	"bytes"
	"strings"
)

const (
	maxLeakScanSize = 10 * 1024 * 1024 // 10MB
	redactionMarker = "<REDACTED_BY_OPCT>"
)

// ScanContentForLeaks scans file content against the embedded leak patterns.
// Returns a list of findings. Skips binary files and files exceeding size limit.
func ScanContentForLeaks(filename string, content []byte) []LeakFinding {
	if len(content) == 0 {
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
	if len(content) == 0 {
		return content, nil
	}

	// Skip binary files (relies on content, not extension)
	if isBinary(content) {
		return content, nil
	}

	contentLower := bytes.ToLower(content)

	// Early exit: if no keywords match ANY pattern, skip expensive processing
	hasAnyKeyword := false
	for i := range leakPatterns {
		if keywordMatch(contentLower, leakPatterns[i].Keywords) {
			hasAnyKeyword = true
			break
		}
	}
	if !hasAnyKeyword {
		return content, nil
	}

	var findings []LeakFinding

	// First pass: collect all matches across all patterns
	type redactSpan struct {
		start int
		end   int
	}
	var allSpans []redactSpan

	for i := range leakPatterns {
		p := &leakPatterns[i]

		if !keywordMatch(contentLower, p.Keywords) {
			continue
		}

		// Find all matches with submatches (for patterns with capture groups)
		// Search on ORIGINAL content, not modified
		matches := p.Regex.FindAllSubmatchIndex(content, -1)
		if matches == nil {
			continue
		}

		// Record finding for logging (without expensive line-by-line search)
		findings = append(findings, LeakFinding{
			File:    filename,
			Pattern: p.Description,
			Line:    0, // Line number detection is too expensive, skip it
		})

		// Collect redaction spans from this pattern
		for _, match := range matches {
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

			// Preserve trailing newline/whitespace
			if end > start && (content[end-1] == '\n' || content[end-1] == '\r') {
				end--
			}

			allSpans = append(allSpans, redactSpan{start, end})
		}
	}

	// If no matches found, return original content
	if len(allSpans) == 0 {
		return content, findings
	}

	// Second pass: apply all redactions
	// Sort spans by start position (simple bubble sort for small number of spans)
	for i := 0; i < len(allSpans)-1; i++ {
		for j := 0; j < len(allSpans)-i-1; j++ {
			if allSpans[j].start > allSpans[j+1].start {
				allSpans[j], allSpans[j+1] = allSpans[j+1], allSpans[j]
			}
		}
	}

	// Build redacted content by copying segments between redactions
	marker := []byte(redactionMarker)
	var result []byte
	lastEnd := 0

	for _, span := range allSpans {
		// Skip overlapping spans (shouldn't happen but be safe)
		if span.start < lastEnd {
			continue
		}

		// Copy content before this redaction
		result = append(result, content[lastEnd:span.start]...)

		// Add redaction marker
		result = append(result, marker...)

		// Check if we need to preserve trailing newline
		if span.end < len(content) && (content[span.end] == '\n' || content[span.end] == '\r') {
			result = append(result, content[span.end])
			lastEnd = span.end + 1
		} else {
			lastEnd = span.end
		}
	}

	// Copy remaining content after last redaction
	result = append(result, content[lastEnd:]...)

	return result, findings
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
