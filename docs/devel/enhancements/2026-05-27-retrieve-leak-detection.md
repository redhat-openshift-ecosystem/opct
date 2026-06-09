# Enhancement: Add leak detection and redaction to `opct retrieve`

**Created:** 2026-05-27  
**Last Updated:** 2026-06-09  
**Status:** In Progress  
**JIRA:** OPCT-423

---

## Revision History

| Date | Author | Changes |
|------|--------|---------|
| 2026-05-27 | Marco Braga | Initial enhancement proposal - warn-only mode |
| 2026-06-05 | Marco Braga | **Scope change:** Updated to redact-by-default with `<REDACTED_BY_OPCT>` marker, DEBUG logging, and `--debug-only-skip-redact` flag |
| 2026-06-09 | Marco Braga | **Architecture change:** Replace SPDY with WebSocket executor, two-phase retrieve (download to disk, then scan). Addresses [vmware-tanzu/sonobuoy#2032](https://github.com/vmware-tanzu/sonobuoy/issues/2032) |

---

## Context

The `opct retrieve` command downloads conformance results from the cluster and saves them as a tar.gz archive. Currently it applies file-level patches (redacting `internalRegistryPullSecret`) and removes large unnecessary files (`packagemanifests`). However, there's no content-based scanning for leaked credentials — AWS keys, SSH private keys, OpenShift tokens, etc. could be present in must-gather logs, resource dumps, or config files.

**Goal:** Embed high-priority leak patterns from [leaktk/patterns](https://github.com/leaktk/patterns) directly into the existing tarball processing pipeline in `cleaner.go`, **automatically redacting** sensitive data during retrieve. Archives are sanitized by default with no sensitive information exposed.

### Sonobuoy SPDY Deprecation

Sonobuoy's `RetrieveResults()` uses `remotecommand.NewSPDYExecutor`, which is deprecated since Kubernetes 1.31. The SPDY protocol causes transient failures (`unexpected EOF`, `non-zero data after tar EOF`) when scanning inline on the network stream. See [vmware-tanzu/sonobuoy#2032](https://github.com/vmware-tanzu/sonobuoy/issues/2032).

**Solution:** Replace with a custom `downloadFromPod()` using `remotecommand.NewWebSocketExecutor` (primary) with `NewSPDYExecutor` fallback via `NewFallbackExecutor` from `k8s.io/client-go`. The archive is downloaded to a temp file first, then scanned from disk — separating unreliable network I/O from reliable disk I/O.

## Behavior: Redact-by-default

### Default Mode (Production)
- Scans all files during retrieve streaming
- **Automatically replaces** detected sensitive patterns with `<REDACTED_BY_OPCT>`
- Logs findings at **DEBUG level** (hidden unless `--log-level=debug`)
- Archive contains **no sensitive information** by default
- Safe for distribution to Red Hat partners

### Debug Mode (Development Only)
- `--debug-only-skip-redact` flag available for troubleshooting
- **WARNING:** Displays prominent warning that archives may contain sensitive data
- Should NEVER be used for archives shared externally
- Useful for validating detection accuracy during development

## Approach: Embedded patterns (no external dependency)

LeakTK is CLI-only (pre-v1.0, no stable Go library). Instead, we'll extract ~12 high-value regex patterns from leaktk/patterns TOML and compile them as Go structs. This integrates directly into the existing `processTarHeader()` in `cleaner.go` — zero external dependencies, no subprocess overhead.

## Implementation

### 1. New file: `internal/cleaner/leakpatterns.go`

Define leak pattern structs and the curated pattern set:

```go
type LeakPattern struct {
    ID          string
    Description string
    Regex       *regexp.Regexp
    Keywords    []string       // pre-filter optimization
}

type LeakFinding struct {
    File        string
    Pattern     string
    Line        int
    MatchLength int            // length of detected secret (for redaction)
}
```

Each pattern must include a comment referencing the source:
```go
// Source: https://github.com/leaktk/patterns
// Pattern ID: sOZiHxUBVFc (leaktk v8.27.0)
```

**Priority patterns to embed (~12 rules):**

| Category | Pattern ID | Description |
|----------|-----------|-------------|
| OpenShift/K8s | `sOZiHxUBVFc` | OpenShift User Token |
| OpenShift/K8s | `vAAom0bPHy8` | Kubernetes Service Account JWT |
| OpenShift/K8s | `gpfGmO3HH64` | Container Registry Authentication |
| AWS | `LAJoYTdoQH4` | AWS IAM Unique Identifier |
| AWS | `9j_rmwDeioM` | AWS Secret Access Key |
| Azure | `zl044yuux24` | Azure AD Client Secret |
| GCP | `HysINeDft8k` | GCP API Key |
| Private keys | `ePK9whPQPpY` | Private Key (PEM header) |
| Generic | `_-9w6-yrc-4` | Generic Secret (key=value quoted) |
| Generic | `hG-qMjbXGro` | Generic Secret (key=value unquoted) |
| GitHub | `gODCNuGzuKQ` | GitHub Personal Access Token |
| GitHub | `kX_PwM0MFvE` | GitHub Fine-Grained PAT |

Fetch the actual regex patterns from https://github.com/leaktk/patterns/blob/main/patterns/gitleaks/8.27.0/98-general.toml at implementation time.

### 2. New file: `internal/cleaner/leakscanner.go`

Scanner function that detects AND redacts sensitive patterns:

```go
// ScanContentForLeaks detects potential leaks (read-only, for logging)
func ScanContentForLeaks(filename string, content []byte) []LeakFinding

// ScanAndRedactLeaks detects and redacts sensitive patterns in content
func ScanAndRedactLeaks(filename string, content []byte) (redacted []byte, findings []LeakFinding)
```

**Redaction behavior:**
- Replace entire matched pattern with `<REDACTED_BY_OPCT>`
- Preserve file structure (same number of lines)
- Maintain readability of surrounding context
- Example: `AWS_KEY=AKIAIOSFODNN7EXAMPLE` → `AWS_KEY=<REDACTED_BY_OPCT>`

**Scanner optimizations:**
- Skip binary files (check for null bytes in first 512 bytes)
- Skip files > 10MB (avoid scanning large tarballs-within-tarballs)
- Apply keyword pre-filter before regex (optimization from leaktk)
- Scan line-by-line for regex matches
- Return findings with file path, pattern description, line number

### 3. Modify: `internal/cleaner/cleaner.go`

In `processTarHeader()`, after reading file content:

```go
// After reading content, before writing to tarWriter:
if header.Size <= int64(maxLeakScanSize) {
    // Scan and redact sensitive data
    redactedContent, findings := ScanAndRedactLeaks(header.Name, content)
    
    // Log findings at DEBUG level (hidden by default)
    if len(findings) > 0 {
        log.Debugf("Leak scan: %d finding(s) in %s", len(findings), header.Name)
        for _, f := range findings {
            if f.Line > 0 {
                log.Debugf("  %s:%d — %s", f.File, f.Line, f.Pattern)
            } else {
                log.Debugf("  %s — %s", f.File, f.Pattern)
            }
        }
    }
    
    // Use redacted content for archive
    content = redactedContent
}

// Write redacted content to tarWriter
```

**Key changes:**
- Content is redacted BEFORE being written to the output archive
- Logging is DEBUG level (not WARN) — hidden unless user explicitly enables debug logging
- No sensitive data in final archive by default

### 4. Add CLI flag: `--debug-only-skip-redact`

Add flag to `retrieve` command for development/debugging:

```go
var skipRedact bool
cmd.Flags().BoolVar(&skipRedact, "debug-only-skip-redact", false, 
    "Skip redaction of detected sensitive data (WARNING: NOT RECOMMENDED - archives may contain secrets)")
```

When flag is used:
```
WARNING: --debug-only-skip-redact enabled
WARNING: Sensitive data will NOT be redacted from the archive
WARNING: DO NOT share this archive externally - it may contain credentials
```

### 5. Summary at end of retrieve (DEBUG level only)

After archive is saved, if `--log-level=debug`:

```
DEBUG Leak scan completed: 3 findings redacted in 2 files
DEBUG   config/auth.json:12 — Container Registry Authentication → <REDACTED_BY_OPCT>
DEBUG   secrets/kubeconfig — Kubernetes Service Account JWT → <REDACTED_BY_OPCT>
DEBUG   logs/installer.log:456 — AWS Secret Access Key → <REDACTED_BY_OPCT>
INFO  Results saved to opct_202606051230_a1b2c3d4.tar.gz
```

In normal operation (no debug logging):
```
INFO Collecting results...
INFO Results saved to opct_202606051230_a1b2c3d4.tar.gz
```

### 6. Tests: `internal/cleaner/leakscanner_test.go`

Unit tests must verify:
- Each pattern detects known test vectors correctly
- Redaction replaces sensitive data with `<REDACTED_BY_OPCT>`
- Redacted content does NOT contain original secret
- Binary file skip works
- Large file skip works
- Line structure preserved after redaction
- Multiple secrets on same line are all redacted

E2E tests must verify:
- Running `opct adm cleaner` produces archive with no sensitive data
- Output archive can be inspected and contains `<REDACTED_BY_OPCT>` markers
- Original sensitive data is NOT present in output

## Files to modify

| File | Change |
|------|--------|
| `internal/cleaner/leakpatterns.go` | New — pattern definitions |
| `internal/cleaner/leakscanner.go` | New — scanning + redaction logic |
| `internal/cleaner/leakscanner_test.go` | New — tests including redaction verification |
| `internal/cleaner/cleaner.go` | Hook scanner into `processTarHeader()`, apply redaction |
| `pkg/retrieve/retrieve.go` | WebSocket+SPDY fallback executor, two-phase retrieve, `--debug-only-skip-redact` flag |
| `.github/workflows/e2e.yaml` | Update E2E tests to verify redaction |

## Performance considerations

- **Keyword pre-filter:** Each pattern has keywords (e.g., `["akia", "asia"]` for AWS keys). Check if any keyword is present in the file content (case-insensitive) before running the full regex. This eliminates >99% of regex invocations.
- **Binary skip:** Don't scan binary files (tar archives, images, compressed data).
- **Size limit:** Skip files > 10MB.
- **Existing pipeline:** The tarball is already read file-by-file in memory. Scanning adds a regex pass on the already-loaded bytes — no additional I/O.
- **Redaction overhead:** String replacement is fast (single pass), minimal overhead compared to regex matching.

### Measured performance (OCP 4.22.0-ec.5, 109.5 MB archive, 1190 files)

| Metric | Before (SPDY inline) | After (WebSocket two-phase) |
|--------|---------------------|-----------------------------|
| **Success rate** | ~0% (10 retries, all failed) | 100% (first attempt) |
| **Total time** | 5m15s (all failures) | **26s** |
| **Download** | N/A (inline) | 15s |
| **Scan/Redact** | N/A (hung or crashed) | 10s (17 redactions) |
| **Memory (RSS)** | 227 MB | 55 MB |

## Security model

### Threat model
**Problem:** OPCT archives may contain sensitive credentials accidentally captured in:
- Must-gather cluster dumps
- Pod logs
- Installation manifests
- Config files
- Error messages

**Risk:** Partners sharing archives with Red Hat could inadvertently expose:
- Cloud provider credentials (AWS, Azure, GCP)
- Cluster admin tokens
- Private keys
- Registry authentication

**Solution:** Automatic redaction during retrieve ensures archives are safe-by-default.

### Design principles
1. **Secure by default:** Redaction happens automatically, no opt-in required
2. **Defense in depth:** Multiple layers (JSON patches + file removal + content scanning)
3. **Fail-closed:** If scan fails, retrieve fails — no unredacted archives are produced
4. **Transparency:** DEBUG logging shows what was redacted
5. **Escape hatch:** `--debug-only-skip-redact` for troubleshooting (with prominent warnings)

### Redaction marker choice
`<REDACTED_BY_OPCT>` was chosen because:
- Clearly indicates intentional redaction (not corruption)
- Shows redaction was performed by OPCT (traceable)
- Easy to grep/search for in archives
- Maintains file structure for debugging
- Cannot be confused with real data
- Short and clear

## Future enhancements

### Phase 2 (post-MVP):
- Redaction summary report (JSON output with what was redacted)
- Additional patterns (database credentials, API tokens)
- Configurable redaction marker (env var override)
- Performance metrics logging

### Phase 3 (future):
- Custom pattern support (user-provided regex)
- Allowlist for false positives
- Differential privacy techniques (k-anonymity for cluster IDs)

## Verification

1. `make build && make test` — all tests pass
2. Run retrieve with debug: `./build/opct-linux-amd64 retrieve --log-level=debug`
3. Verify archives contain `<REDACTED_BY_OPCT>` markers
4. Verify archives do NOT contain original secrets
5. Test `--debug-only-skip-redact` flag shows warnings
6. Verify normal retrieve (no debug) shows no leak messages
7. Benchmark: compare retrieve time with/without scanning

## Package header

The `leakpatterns.go` file header must reference the upstream pattern source:
```go
// Package cleaner provides leak detection patterns for scanning OPCT archives.
//
// Patterns are sourced from the leaktk/patterns project:
//   https://github.com/leaktk/patterns
//
// To update patterns, fetch the latest merged TOML from:
//   https://github.com/leaktk/patterns/blob/main/patterns/gitleaks/8.27.0/98-general.toml
//
// LeakTK documentation:
//   https://github.com/leaktk/leaktk
//   https://github.com/leaktk/leaktk/blob/main/docs/scan.md
```
