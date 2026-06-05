package cleaner

import (
	"bytes"
	"strings"
)

const maxLeakScanSize = 10 * 1024 * 1024 // 10MB

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
