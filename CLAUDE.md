# CLAUDE.md - Development Instructions for AI Assistants

This document provides structured instructions for common development activities on the OPCT project, designed for AI assistants (like Claude) to execute consistently and correctly.

## Table of Contents

- [Project Structure](#project-structure)
- [Development Tasks](#development-tasks)
  - [Go Version Bump](#go-version-bump)
  - [Dependency Management](#dependency-management)
  - [Adding Retry Logic to Validations](#adding-retry-logic-to-validations)
- [Release Process](#release-process)
  - [Release Overview](#release-overview)
  - [Release Invariants](#release-invariants)
  - [Release Checklist](#release-checklist)
- [Validation Procedures](#validation-procedures)
- [Web UI Report Development](#web-ui-report-development)
- [AI Agent Ecosystem](#ai-agent-ecosystem)
- [Contributing Guidelines](#contributing-guidelines)

---

## Project Structure

OPCT is organized into the following key components:

- **CLI (`cmd/opct`)**: Client-side utility to orchestrate conformance workflows
- **Internal packages (`internal/`)**: Core business logic and utilities
- **Public packages (`pkg/`)**: Public APIs and client interfaces
- **Plugins**: Container-based workflow steps (separate repository)
- **Documentation (`docs/`)**: User and developer documentation

### Key Directories

```
.
├── .github/workflows/      # GitHub Actions CI/CD workflows
├── cmd/opct/              # CLI entrypoint
├── internal/              # Internal packages
├── pkg/                   # Public packages
├── hack/                  # Build scripts and Containerfile
├── docs/                  # Documentation
│   ├── devel/            # Developer guides
│   └── user/             # User guides
└── test/                  # Test utilities
```

---

## Development Tasks

### Go Version Bump

**When to use**: Update project to use a newer Go version available in the build environment.

**Command pattern**: "Update to Go 1.X.Y" or "Bump Go version to 1.X.Y"

#### Files to Update

1. **`go.mod`**
   - Update `go` directive (e.g., `go 1.25.0`)
   - **Do not use `toolchain` directive** - it can cause CI compatibility issues
   - Use only the `go` directive for maximum compatibility with CI tools

2. **`.github/workflows/*.yaml` (ALL workflow files)**
   - Search for ALL files with `GO_VERSION`: `grep -rn "GO_VERSION" .github/workflows/`
   - Update `GO_VERSION` environment variable in:
     - `go.yaml` (line ~16)
     - `pre_linters.yaml` (line ~19)
     - `pre_reviewer.yaml` (line ~15)
     - `e2e.yaml` (line ~13)
   - Format: `GO_VERSION: 1.25` (major.minor only)
   - **Important**: Always search for ALL occurrences, as new workflows may be added

3. **`hack/Containerfile`**
   - Update builder image: `FROM docker.io/golang:1.25-alpine AS builder`
   - Check base image for latest stable: `FROM quay.io/fedora/fedora-minimal:XX`
   - To find latest Fedora stable: Check [Fedora releases](https://fedoraproject.org/wiki/Releases)

4. **`hack/*.sh` scripts (containerized Go tools)**
   - Search for ALL golang image references: `grep -rn "golang:1\." hack/`
   - Update in:
     - `hack/go-imports.sh` (line ~14): `docker.io/golang:1.25`
     - `hack/go-staticcheck.sh` (line ~12): `docker.io/golang:1.25`
   - Format: `docker.io/golang:1.25` (major.minor only)
   - **Important**: Always search for ALL occurrences, as new scripts may be added

#### Procedure

```bash
# 1. Check current environment Go version
go version  # e.g., go version go1.25.4 linux/amd64

# 2. Update go.mod
# - Set go directive to major.minor.patch (e.g., 1.25.0)
# - Do not add toolchain directive (for CI compatibility)

# 3. Update ALL .github/workflows/*.yaml files
# - Search for all GO_VERSION references: grep -rn "GO_VERSION" .github/workflows/
# - Update in go.yaml, pre_linters.yaml, pre_reviewer.yaml, e2e.yaml
# - Set GO_VERSION to major.minor only (e.g., 1.25)

# 4. Update hack/Containerfile
# - Update golang builder image to match major.minor
# - Optionally update fedora-minimal base image

# 5. Update hack/*.sh scripts
# - Search for all golang image references: grep -rn "golang:1\." hack/
# - Update docker.io/golang images in hack/go-imports.sh and hack/go-staticcheck.sh

# 6. Resolve dependencies
go mod tidy

# 7. Validate changes
make test
make vet
make test-lint  # May show pre-existing YAML lint issues - that's OK

# 8. Commit changes
git add go.mod go.sum .github/workflows/*.yaml hack/Containerfile hack/*.sh
git commit -m "chore: bump Go version to X.Y.Z

Updated Go version from X.Y.Z to A.B.C to use the latest
Go version available in the build environment.

Changes:
- Updated go directive to A.B.C in go.mod
- Removed toolchain directive (for CI compatibility)
- Updated CI workflows GO_VERSION to A.B (4 workflow files)
- Updated hack/Containerfile golang image to A.B-alpine
- Updated hack/*.sh golang images to A.B
- Resolved dependencies with go mod tidy

Validation:
- ✅ make test - all tests passed
- ✅ make vet - no issues found

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

#### Expected Results

- **Build**: `make build` should succeed
- **Tests**: All existing tests should pass
- **Linting**: Go code should pass linting (YAML workflow linting may have pre-existing issues)
- **CI**: Workflows should use new Go version

#### Common Issues

- **YAML linting errors**: These are typically pre-existing issues in `.github/workflows/*.yaml` files and unrelated to Go version changes
- **Dependency conflicts**: Run `go mod tidy` to resolve; if issues persist, check for incompatible dependencies
- **golangci-lint compatibility**: The project uses golangci-lint v2.6.2+ which supports Go 1.25+
  - If upgrading to Go 1.26+, ensure golangci-lint version supports it
  - Check golangci-lint releases: https://github.com/golangci/golangci-lint/releases
  - Update `GOLANGCI_LINT_VERSION` in `.github/workflows/pre_linters.yaml` if needed

---

### Dependency Management

**When to use**: Update, add, or remove Go dependencies.

#### Reducing Dependencies (SBOM Optimization)

**Reference commit**: `26e8975` - "refactor: replace pkg/errors with stdlib fmt.Errorf"

##### Replace `github.com/pkg/errors` with stdlib

**Common pattern**: Replace external error handling libraries with Go's built-in error wrapping.

```go
// Before
import "github.com/pkg/errors"
errors.New("message")
errors.Wrap(err, "context")
errors.Wrapf(err, "context: %v", value)

// After
import "fmt"
fmt.Errorf("message")
fmt.Errorf("context: %w", err)
fmt.Errorf("context %v: %w", value, err)
```

**Files typically affected**:
- `pkg/cmd/**/*.go`
- `pkg/run/**/*.go`
- `internal/**/*.go`

**Validation steps**:
1. Search for all `errors.New`, `errors.Wrap`, `errors.Wrapf` usages
2. Replace with `fmt.Errorf` equivalents using `%w` for wrapping
3. Remove `"github.com/pkg/errors"` import
4. Remove from `go.mod` direct dependencies
5. Run `go mod tidy`
6. Run `make test` and `make build`

##### Upgrade YAML library (v2 → v3)

**Pattern**: Consolidate duplicate YAML libraries.

```go
// Before
import "gopkg.in/yaml.v2"

// After
import "gopkg.in/yaml.v3"
```

**Note**: API is mostly compatible; check for any breaking changes in complex YAML operations.

#### Dependency Analysis Commands

```bash
# Count total dependencies
go list -m all | wc -l

# Find direct dependencies
grep -A 100 "^require (" go.mod | grep -v "^)"

# Check why a dependency is needed
go mod why <package>

# Find duplicate/similar dependencies
go mod graph | grep <pattern>

# View dependency tree for a specific package
go mod graph | grep "^github.com/your/project " | head -30
```

#### Bumping Kubernetes and OpenShift Client Libraries

**When to use**: Update Kubernetes and OpenShift client libraries to the latest stable versions.

**Command pattern**: "Bump the dependencies to the latest stable versions" or "Update k8s and openshift client libraries"

##### OpenShift Client Libraries

OpenShift uses branch-based versioning with the pattern `release-X.Y` (e.g., `release-4.22`).

**Repositories to check**:
- [`github.com/openshift/api`](https://github.com/openshift/api/branches) - OpenShift API definitions
- [`github.com/openshift/client-go`](https://github.com/openshift/client-go/branches) - OpenShift Go client

**How to find the latest version**:
1. Visit the repository's branches page (append `/branches` to the GitHub URL)
2. Look for the highest numbered `release-X.Y` branch (e.g., `release-4.22` is newer than `release-4.21`)
3. The latest branch represents the latest stable OpenShift version

**How to update**:
```bash
# Check current version in go.mod
grep "github.com/openshift/api" go.mod
grep "github.com/openshift/client-go" go.mod

# Example output (current):
# github.com/openshift/api v0.0.0-20250225181102-cb44c196e68f // github.com/openshift/api@release-4.18

# Update to latest stable branch (e.g., release-4.22)
go get github.com/openshift/api@release-4.22
go get github.com/openshift/client-go@release-4.22

# Resolve dependencies
go mod tidy
```

##### Kubernetes Client Libraries

Kubernetes uses semantic versioning with the pattern `v0.X.Y` which corresponds to Kubernetes `v1.X.Y`.

**Repositories to check**:
- [`k8s.io/api`](https://github.com/kubernetes/api/branches) - Kubernetes API definitions
- [`k8s.io/apimachinery`](https://github.com/kubernetes/apimachinery/branches) - Kubernetes API machinery
- [`k8s.io/client-go`](https://github.com/kubernetes/client-go/branches) - Kubernetes Go client
- [`k8s.io/utils`](https://github.com/kubernetes/utils/branches) - Kubernetes utility functions

**Version mapping**:
- Kubernetes `v1.34.x` → client-go `v0.34.x`
- Kubernetes `v1.35.x` → client-go `v0.35.x`

**How to find the latest version**:
1. Visit the repository's branches page
2. Look for the highest numbered `release-X.Y` branch (e.g., `release-1.34`)
3. The corresponding client library version is `v0.X.Y` (e.g., `v0.34.x`)

**How to update**:
```bash
# Check current version in go.mod
grep "k8s.io/" go.mod | grep -E "(api|apimachinery|client-go|utils)"

# Example output (current):
# k8s.io/api v0.32.2
# k8s.io/apimachinery v0.32.2
# k8s.io/client-go v0.32.2

# Update to latest stable version (e.g., v0.34.x)
# Note: Use @latest to get the latest patch version in the 0.34 series
go get k8s.io/api@v0.34.0
go get k8s.io/apimachinery@v0.34.0
go get k8s.io/client-go@v0.34.0
go get k8s.io/utils@latest

# Resolve dependencies and update to latest patch versions
go mod tidy
```

##### Complete Update Procedure

```bash
# 1. Check latest stable versions
# OpenShift: Check https://github.com/openshift/api/branches for latest release-X.Y
# Kubernetes: Check https://github.com/kubernetes/client-go/branches for latest release-X.Y

# 2. Update OpenShift libraries (e.g., to release-4.22)
go get github.com/openshift/api@release-4.22
go get github.com/openshift/client-go@release-4.22

# 3. Update Kubernetes libraries (e.g., to v0.34.0)
go get k8s.io/api@v0.34.0
go get k8s.io/apimachinery@v0.34.0
go get k8s.io/client-go@v0.34.0
go get k8s.io/utils@latest

# 4. Resolve all dependencies
go mod tidy

# 5. Validate changes
make test
make vet
make build

# 6. Verify updated versions in go.mod
grep "github.com/openshift/api" go.mod
grep "github.com/openshift/client-go" go.mod
grep "k8s.io/" go.mod | grep -E "(api|apimachinery|client-go|utils)"

# 7. Commit changes
git add go.mod go.sum
git commit -m "chore: bump k8s and openshift client libraries to latest stable

Updated client library dependencies:
- OpenShift API: release-4.XX → release-4.YY
- OpenShift client-go: release-4.XX → release-4.YY
- k8s.io/api: v0.XX.X → v0.YY.Z
- k8s.io/apimachinery: v0.XX.X → v0.YY.Z
- k8s.io/client-go: v0.XX.X → v0.YY.Z
- k8s.io/utils: updated to latest

Validation:
- ✅ make test - all tests passed
- ✅ make vet - no issues found
- ✅ make build - successful

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

##### Version Compatibility Notes

- **OpenShift and Kubernetes versions should be compatible**: OpenShift 4.22 is based on Kubernetes 1.34, so updating both together is recommended
- **Always update to stable releases**: Use release branches (not master/main) for production dependencies
- **Test thoroughly after updates**: Client library updates may introduce API changes that require code modifications

---

### Adding Retry Logic to Validations

**When to use**: Add resilient retry/timeout logic to pre-run validation checks that may experience transient failures in ephemeral environments (like CI).

**Reference implementation**: Cluster Operator validation enhancement (commit example)

#### Background

The OPCT CLI performs pre-run validations before executing the conformance workflow. These validations check cluster readiness, including:
- Cluster Operator stability
- Image Registry state
- MachineConfigPool status
- Node configuration

In ephemeral environments (especially CI), resources may not be immediately ready, causing false-positive validation failures.

#### Generic Retry Configuration

OPCT provides generic retry/timeout configuration in `RunOptions` that applies to **all pre-run validations**:

```go
// In pkg/run/run.go
type RunOptions struct {
    // ...
    validationTimeout       int  // Total timeout for ALL pre-run validations (seconds)
    validationRetryInterval int  // Interval between retry attempts (seconds)
    // ...
}

const (
    defaultValidationTimeoutSeconds       = 600  // 10 minutes
    defaultValidationRetryIntervalSeconds = 10   // 10 seconds
)
```

**Design principles**:
- ✅ **Generic**: Single configuration for all validations (not per-validation)
- ✅ **Isolated**: Separate from Sonobuoy workflow timeout (`timeout` field)
- ✅ **Hidden**: Advanced flags hidden from users but available for CI tuning

#### Implementation Pattern

**Step 1: Create a wait function with retry logic**

```go
// Example: waitForClusterOperators in pkg/run/run.go:628-682
func waitForClusterOperators(ctx context.Context, configClient coclient.Interface, retryInterval time.Duration) error {
    log.Info("Waiting for cluster operators to become ready...")

    var lastErrors []error
    attempt := 0

    // Use PollUntilContextCancel to respect context deadline
    err := utilwait.PollUntilContextCancel(ctx, retryInterval, true, func(ctx context.Context) (bool, error) {
        attempt++

        // Perform the check
        errs := checkClusterOperators(configClient)

        if len(errs) == 0 {
            log.Info("All cluster operators are ready")
            return true, nil  // Success
        }

        lastErrors = errs

        // Log appropriately
        if attempt == 1 {
            log.Warnf("Cluster operators are not ready yet (attempt %d), will retry every %s", attempt, retryInterval)
        } else {
            log.Debugf("Cluster operators still not ready (attempt %d): %d operator(s) not in ready state", attempt, len(errs))
        }

        return false, nil  // Continue polling
    })

    // Handle timeout
    if err == context.DeadlineExceeded {
        log.Errorf("Timeout waiting for cluster operators after %d attempts", attempt)
        return fmt.Errorf("timeout waiting for cluster operators: %d still not ready after %d attempts: %v", len(lastErrors), attempt, lastErrors)
    }

    return err
}
```

**Step 2: Update validation function to use retry logic**

```go
// Example: validateClusterOperators in pkg/run/validations.go:123-152
func validateClusterOperators(r *RunOptions, restConfig *rest.Config) []error {
    var result []error

    // Create client
    oc, err := coclient.NewForConfig(restConfig)
    if err != nil {
        return []error{err}
    }

    // Create context with timeout from configuration
    ctx, cancel := context.WithTimeout(context.Background(),
        time.Duration(r.validationTimeout)*time.Second)
    defer cancel()

    // Wait with retry logic
    retryInterval := time.Duration(r.validationRetryInterval) * time.Second
    err = waitForClusterOperators(ctx, oc, retryInterval)

    if err != nil {
        if r.devSkipChecks {
            log.Warnf("DEVEL MODE: Skipping validation error: %v", err)
        } else {
            result = append(result, fmt.Errorf("operators are not in ready state: %w", err))
        }
    }

    return result
}
```

#### Key Implementation Details

**Use `PollUntilContextCancel`, not `PollUntilContextTimeout`**:
- ✅ `PollUntilContextCancel(ctx, interval, immediate, func)` - respects context deadline
- ❌ `PollUntilContextTimeout(ctx, interval, timeout, immediate, func)` - requires explicit timeout parameter

**Logging best practices**:
- First attempt: Log as **WARN** with retry info
- Subsequent attempts: Log as **DEBUG** to reduce noise
- Success: Log as **INFO**
- Timeout: Log as **ERROR** with details of what failed

**Error handling**:
- Check for `context.DeadlineExceeded` to detect timeout
- Preserve last error state for detailed reporting
- Use `%w` for error wrapping to maintain error chains

#### CLI Usage

**Default behavior** (automatic):
```bash
opct run
# Uses 10-minute timeout, 10-second retry interval
```

**CI with custom timeout**:
```bash
opct run --validation-timeout=900 --validation-retry-interval=15
# 15 minutes total, 15 seconds between retries
```

#### Applying to Other Validations

The same pattern can be applied to:
- `validateImageRegistry` - Registry reconciliation may take time
- `validateMachineConfigPool` - MCP updates may be in progress
- `validateContainerImagesAccessibility` - Network-dependent checks

**Not recommended for**:
- `validateDedicatedNode` - Node labels are static
- `validateOpctNamespace` - Quick existence check

---

## Release Process

**Authoritative runbook: the `opct-release` skill** (`.claude/skills/opct-release/SKILL.md`).
It covers release candidates and GA, both repositories, the `openshift/release`
CI image bump, the GitHub release-page fixes, and recovery from a failed tag
build. Follow it rather than improvising from this section.

**Background**: [docs/devel/release.md](docs/devel/release.md) (plugin build pipeline),
[docs/devel/update.md](docs/devel/update.md).

### Release Overview

Two repositories ship together:

1. **OPCT CLI** ([redhat-openshift-ecosystem/opct](https://github.com/redhat-openshift-ecosystem/opct)) — binaries + `quay.io/opct/opct`
2. **OPCT Plugins** ([redhat-openshift-ecosystem/opct-plugins](https://github.com/redhat-openshift-ecosystem/opct-plugins)) — `quay.io/opct/plugin-openshift-tests`, `plugin-artifacts-collector`, `must-gather-monitoring`

**Release branches**: `release-X.Y` in **both** repos — no `v` prefix (e.g. `release-0.7`).

**Version format**: `vX.Y.Z` for GA, `vX.Y.Z-rc.N` for release candidates.

### Release Invariants

These are enforced by CI or by hard-won experience; the skill explains each in
context.

- **Plugins are released before the CLI.** The `verify-images` job in
  `.github/workflows/go.yaml` checks that every plugin tag pinned in
  `pkg/types.go` already exists on quay.
- **`main` is permanently `:latest`.** All four image constants in `pkg/types.go`
  stay at `:latest` on main. Pinning happens only on a release branch, where CI
  **blocks** `:latest`. (This replaced the old pre-OPCT-437 "version bump PR to
  main" flow as of v0.6.6 — if you find docs describing that flow, they are stale.)
- **Merge into release branches, never rebase.** A rebase rewrites history and
  orphans previously tagged commits; this already happened to `v0.6.4-rc.0` and
  `v0.6.4-rc.1`.
- **Tags are manual, annotated, and pinned to an explicit SHA**
  (`git tag -a vX.Y.Z -m "..." <full-sha>`). There is no release automation.
- **Minor vs patch**: a minor bump creates a fresh `release-X.Y` from main (no
  merge PR needed to seed it); a patch bump merges main into the existing branch
  via PR, expecting a `pkg/types.go` conflict.
- **Bumping `OPCT_CLI_IMAGE` in `openshift/release`** (3 step-registry workflow
  files, fork-based PR, signed commits, `/pj-rehearse` + ack) is part of the
  release, not a follow-up.
- **Release-branch fixes need a PR, a Jira issue, and a path to `main`.** A
  direct-pushed, untracked lint hotfix on `release-0.6` broke the `v0.7.0-rc.0`
  tag build a year later.
- **RC releases** keep `prerelease: true` and leave the Latest badge on the
  previous GA.

### Release Checklist

**Pre-release**
- [ ] Release Jira issue exists and states the exact version string
- [ ] Blockers triaged — RC blockers and GA blockers are different lists
- [ ] Both repos' `main` green; maintainer agreed on version and branch plan

**Plugins** (first)
- [ ] `release-X.Y` seeded or merged up to date
- [ ] Annotated tag on an explicit SHA, pushed; **tag** CI run green (the branch-push run publishes nothing)
- [ ] All 3 images present on quay

**CLI** (second)
- [ ] PR to `release-X.Y` pinning all 4 constants in `pkg/types.go`; `verify-images` green
- [ ] Merged (merge commit), tag pushed on the merge SHA, tag CI green
- [ ] `quay.io/opct/opct:<version>` present; GitHub release has binaries

**CI and close-out**
- [ ] `openshift/release` PR merged with rehearsals acked
- [ ] Release notes rewritten from the tag message; prerelease/Latest flags correct for RC vs GA
- [ ] Periodics observed on the new image, results recorded on the ticket

---

## Validation Procedures

### Standard Validation Checklist

Before committing any changes, run:

```bash
# 1. Resolve dependencies
go mod tidy

# 2. Build the project
make build

# 3. Run tests
make test

# 4. Run go vet
make vet

# 5. Run linting (optional, may have pre-existing issues)
make test-lint
```

### Expected Test Results

- **make test**: All tests should pass (currently ~10 packages)
- **make vet**: Should complete with no output (no issues)
- **make test-lint**: May show YAML workflow formatting issues (pre-existing)
  - Go code linting is currently commented out in Makefile
  - YAML issues in `.github/workflows/*.yaml` are acceptable if unrelated to changes

### Test Output Interpretation

```bash
# Good test run
ok  	github.com/redhat-openshift-ecosystem/opct/internal/opct/summary	0.014s

# Failed test (requires investigation)
FAIL	github.com/redhat-openshift-ecosystem/opct/internal/opct/summary [build failed]

# No test files (acceptable)
?   	github.com/redhat-openshift-ecosystem/opct/cmd/opct	[no test files]
```

---

## AI Agent Ecosystem

OPCT uses two complementary AI systems for different stages of the SDLC. They share domain knowledge but operate in distinct contexts — avoid duplicating work across both.

### Claude Code Agents and Skills (this repository)

**Scope**: Local developer workflows on the OPCT codebase and CI infrastructure.

| Asset | Location | Purpose |
|-------|----------|---------|
| Agents | `.claude/agents/` | Specialized workflows (CI triage, PR review, web UI dev, general development) |
| Skills | `.claude/skills/` | Reusable step-by-step procedures invoked by agents |
| Marketplace config | `.claude/settings.json` | Enables `openshift-eng/ai-helpers` plugins (`ci:*`, `jira:*` skills) |

**Handles**:
- Building, testing, and validating code changes (`go-validate`, `e2e-quick-test`)
- CI periodic job failure triage (`ci-triage`)
- PR review and code quality checks (`pr-reviewer`)
- Web UI report development (`webui-developer`, `webui-report-test`)
- Jira operations for OPCT project issues (`jira-ops`)

### Chai Bot VCSP Persona (`vcsp_certops`)

**Scope**: OPCT certification review and partner validation.

**Handles**:
- OPCT archive structure validation
- Test result analysis against validation rules (OPCT-001 through OPCT-040)
- Sippy flake data cross-referencing
- Root cause investigation of certification failures
- Structured pass/fail/review-needed verdicts
- Jira routing for certification-related issues

See `docs/review/rules.md` for the full validation rule catalog.

### Boundary: When to Use Which

| Task | Use |
|------|-----|
| Fix a bug in OPCT CLI or report UI | Claude Code (`opct-developer`) |
| Triage a failing periodic CI job | Claude Code (`ci-triage`) |
| Review a pull request | Claude Code (`pr-reviewer`) |
| Validate a partner OPCT archive for certification | Chai Bot VCSP persona |
| Analyze conformance test failures in a submitted archive | Chai Bot VCSP persona |
| Produce certification approve/reject recommendation | Chai Bot VCSP persona |

### CI Triage ↔ VCSP Persona Integration

The `ci-triage` agent classifies periodic CI failures. These classifications map to how the VCSP persona evaluates certification outcomes:

| CI Triage Classification | VCSP Persona Context |
|--------------------------|---------------------|
| `KNOWN_FLAKE` | Test has high flake rate in Sippy — not a certification blocker unless it violates an OPCT validation rule threshold (e.g., OPCT-004 pass ratio) |
| `EXISTING_BUG` | Link to the open Jira issue; VCSP persona may reference the same bug in certification review |
| `INFRA_FAILURE` | CI infrastructure issue (lease timeout, VM provisioning) — not a product or certification failure |
| `NEW_FAILURE` | Real test failure — may indicate an OPCT validation rule violation (OPCT-001 through OPCT-040) requiring VCSP review |

**Feeding triage results into VCSP review**: When a CI triage identifies `NEW_FAILURE` items, include the Prow job URL, failed test names, and Sippy flake rates in the Jira bug description. The VCSP persona can then cross-reference these against validation rules when reviewing partner archives that exhibit the same failures.

### Jira Project Routing

| Failure type | Jira project | Example |
|--------------|-------------|---------|
| OPCT tool bug (CLI, plugins, report UI) | **OPCT** | Report generation crash, plugin timeout |
| OPCT CI infrastructure failure | **OPCT** | Periodic job lease timeout, test harness error |
| Upstream OCP platform bug found during OPCT runs | **OCPBUGS** | Kubernetes conformance test failure in core OCP |
| Certification review finding | **OPCT** | Validation rule violation in partner archive |

**Default for CI triage**: File against **OPCT** (parent OPCT-400). Only route to **OCPBUGS** when the failure is clearly an upstream OpenShift platform issue unrelated to OPCT tooling.

### Bug Filing Policy

All Jira bug creation requires **human approval** before filing — regardless of whether MCP or REST API is available. Agents draft the bug with all fields populated and present it for review. MCP provides the filing mechanism; the user makes the decision.

---

## Contributing Guidelines

### Commit Message Format

Follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>: <description>

[optional body]

[optional footer(s)]
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks (dependency updates, tooling, etc.)

**Examples**:

```
chore: bump Go version to 1.25.0

feat: add support for custom plugin manifests

refactor: replace pkg/errors with stdlib fmt.Errorf

fix: correct error handling in baseline reporting
```

### AI Attribution (Required)

All AI-generated content must be attributed for traceability.

**Git commits** — include the co-author trailer:

```text
Co-Authored-By: Claude <noreply@anthropic.com>
```

**GitHub interactions** (PR descriptions, comments, review replies) — append at the end:

```text
— AI Claude
```

**Jira comments** (via Claude Code agents or skills) — append at the end:

```text
— AI Claude
```

**Chai Bot VCSP persona** — uses its own built-in attribution (automatic Slack thread link appended to Jira comments). Do not add `— AI Claude` to Jira comments created by the VCSP persona; that attribution is handled by Chai Bot.

### Branch Naming

- Jira-key branches: `OPCT-NNN-<description>` (e.g., `OPCT-419-agentic-assets`)
- Feature branches: `feature/<description>`
- Bug fixes: `fix/<description>`
- Development tasks: `dev/<description>`
- Review/testing: `review-<description>`

### Pull Request Guidelines

1. Create a descriptive PR title following Conventional Commits
2. Include a detailed description of changes
3. Reference related issues with `Fixes #123` or `Relates to #123`
4. Ensure all CI checks pass
5. Request review from maintainers

---

## Additional Resources

- **Release Process**: [docs/devel/release.md](docs/devel/release.md)
- **Component Updates**: [docs/devel/update.md](docs/devel/update.md)
- **Developer Guide**: [docs/devel/guide.md](docs/devel/guide.md)
- **User Documentation**: [https://redhat-openshift-ecosystem.github.io/opct/](https://redhat-openshift-ecosystem.github.io/opct/)
- **Contributing Guide**: [CONTRIBUTING.md](CONTRIBUTING.md)

---

## What's Next

This section outlines potential improvements and opportunities for the OPCT project based on lessons learned.

### Short-Term Improvements

**1. Fix golangci-lint-action compatibility**
- **Issue**: v7 action broke tag builds with `only-new-issues` flag
- **Options**:
  - Revert to `golangci/golangci-lint-action@v6` (simple, proven to work)
  - Remove `only-new-issues` flag entirely (lint full codebase on every build)
  - Use `continue-on-error: true` for tag builds (allows release to proceed)
- **Recommended**: Revert to v6 until v7 compatibility is confirmed

**2. Standardize release tag message format**
- Create template for comprehensive changelogs
- Include sections: Bug Fixes, New Features, Enhancements, Dependencies
- Reference PR numbers for traceability
- Document in this file for future releases

**3. Document common CI failures**
- Create troubleshooting guide for CI build failures
- Include common errors and solutions
- Add to developer documentation

### Medium-Term Improvements

**1. Improve CI stability**
- Audit all CI workflows for reliability
- Identify non-critical jobs that can use `continue-on-error`
- Ensure critical failures are clearly distinguished from warnings
- Test workflows on feature branches before merging to main

**2. Release automation considerations**
- **Do not** implement automated tag creation workflows (proven unreliable)
- **Consider** simple release checklist automation (PR templates, issue templates)
- **Focus** on improving manual process documentation and tooling
- **Validate** that any automation is thoroughly tested before adoption

**3. Version management**
- Consider using `VERSION` file in repository root
- Automate version bumps in `pkg/types.go` via script
- Add validation to ensure version consistency across files

### Long-Term Opportunities

**1. Release process tooling**
- Create simple CLI tool for release tasks (`opct-release`)
- Features:
  - Interactive changelog generation from git history
  - Automated version consistency checks
  - Release checklist validation
  - Tag creation with standardized format
- **Important**: Keep tool simple, avoid complex automation

**2. Testing improvements**
- Expand unit test coverage
- Add integration tests for critical workflows
- Implement pre-release testing checklist
- Consider automated smoke tests for releases

**3. Documentation enhancements**
- Create video walkthrough of release process
- Add diagrams for release workflows
- Document rollback procedures
- Expand troubleshooting guides

### Anti-Patterns to Avoid

Based on v0.6.1 experience:

❌ **Complex GitHub Actions workflows for infrequent tasks**
- Releases happen ~monthly, automation overhead not worth it
- Hard to debug when they fail
- Manual process provides better control

❌ **Changing workflows without thorough testing**
- Test on feature branches first
- Validate in dry-run mode
- Document expected behavior

❌ **Over-engineering solutions**
- Simple solutions are better for maintainability
- Don't add conditional logic unless absolutely necessary
- Prefer established patterns over novel approaches

❌ **Force-pushing to shared branches**
- Causes confusion and potential data loss
- Use feature branches and PRs instead
- Only force-push to personal branches

---

## Web UI Report Development

**When to use**: Modify the OPCT web report UI (charts, tables, layout, new pages).

### Architecture

The report is a **Vue.js 2 single-page application** rendered as a Go template.

**Key files**:
- `data/templates/report/report.html` — Main Vue app (Go template delimiters: `[[` / `]]`)
- `data/templates/report/report.css` — Styles
- `internal/report/data.go` — Backend: `SaveResults()` renders templates to `<saveTo>/index.html`
- `internal/openshift/mustgathermetrics/plotly.go` — Generates Plotly-format JSON chart data to `<saveTo>/metrics/`

**CDN libraries loaded in report.html**:
- Vue 2, Bootstrap, BootstrapVue, axios
- Chart.js 4.4.9 + chartjs-adapter-date-fns (time-series X axes)
- Hammer.js + chartjs-plugin-zoom (drag-to-zoom, pan)

**Content rendering pattern**: Each page builds HTML strings in `this.menuBody` and renders via `v-html` directive. Data comes from `opct-report.json` (fetched by axios or embedded via Go template).

### Testing Workflow

```bash
# 1. Build
make build

# 2. Generate report (--skip-server to avoid blocking)
TEST_ID=<result-name>  # e.g., 4.20.0-0.nightly-2026-05-04-230007-20260510-HighlyAvailable-aws-External
./build/opct-linux-amd64 report -s ~/opct/tmp/${TEST_ID}__report --skip-server ~/opct/results/${TEST_ID}.tar.gz

# 3. Serve for browser testing
python3 -m http.server 9090 --directory ~/opct/tmp/${TEST_ID}__report

# 4. Open http://localhost:9090 in browser, test all pages
```

Always test:
- The page you modified
- At least 2-3 other pages (Summary, Checks, Network) to verify no layout regressions
- Browser resize for responsive behavior

### Split-Pane Layout Pattern

Some pages (e.g., etcd) use a split-pane layout with tables on the left and charts on the right.

**Critical rule**: Use `v-if`/`v-else` to conditionally render the split container — **never** use a shared container with conditional CSS classes.

```html
<!-- CORRECT: split pane only exists when menuBodyRight has content -->
<div v-if="menuBodyRight" id="split-container" class="d-flex">
  <div id="panel-left" class="etcd-panel-left overflow-auto">
    <span v-html="menuBody"></span>
  </div>
  <div id="panel-divider" class="etcd-panel-divider"></div>
  <div id="panel-right" class="etcd-panel-right overflow-auto">
    <span v-html="menuBodyRight"></span>
  </div>
</div>
<div v-else class="overflow-auto">
  <span v-html="menuBody"></span>
</div>
```

```html
<!-- WRONG: d-flex affects all pages even when menuBodyRight is empty -->
<div class="d-flex">
  <div :class="menuBodyRight ? 'etcd-panel-left' : ''">
    <span v-html="menuBody"></span>
  </div>
</div>
```

**Why**: The `d-flex` container changes layout behavior for ALL pages. Inline styles set by JavaScript (e.g., drag-resize) persist across page navigations. The `v-if`/`v-else` approach ensures non-split pages use the original full-width layout unchanged.

**Cleanup**: `changeMenuCleanup()` must clear `menuBodyRight` AND reset any inline styles set by the drag-resize handler:

```javascript
changeMenuCleanup() {
  this.menuTitle = '';
  this.menuBody = '';
  this.menuBodyRight = '';
  var left = document.getElementById('panel-left');
  if (left) { left.style.width = ''; left.style.minWidth = ''; }
  var right = document.getElementById('panel-right');
  if (right) { right.style.width = ''; right.style.minWidth = ''; }
},
```

### Adding Charts to a Page

Chart data is served as static JSON files at `./metrics/` (generated during report processing). To add charts to a page:

1. **Define chart metadata** in Vue `data()`:
   ```javascript
   etcdCharts: [
     { id: "etcd-chart-0", path: "./metrics/query_range-etcd-...", title: "Chart Title" },
   ],
   ```

2. **Build canvas placeholders** in `menuBodyRight` (Chart.js uses `<canvas>`, not `<div>`):
   ```javascript
   this.menuBodyRight += `<div class="etcd-chart-card"><canvas id="` + chart.id + `"></canvas></div>`
   ```

3. **Render after DOM update** with `this.$nextTick(() => { this.renderCharts(); })`

4. **Fetch JSON and create Chart.js instances**:
   ```javascript
   axios.get(chart.path).then(resp => {
     let reply = resp.data;
     // reply.data = [{x: [...], y: [...], name: "label", type: "scatter"}]
     // Map Plotly format to Chart.js datasets
     new Chart(canvas, { type: 'line', data: { datasets }, options: { ... } });
   });
   ```

**Chart data format** (Plotly JSON, reused by Chart.js):
- `data[].x` — timestamps as strings (e.g., `"2026-5-9 21:44:8"`, non-padded)
- `data[].y` — float values
- `data[].name` — series label
- Use `parseMetricTimestamp()` for cross-browser safe date parsing

**Available etcd metrics** (defined in `internal/openshift/mustgathermetrics/main.go`):
- `query_range-etcd-disk-fsync-wal-duration-p99.json.gz`
- `query_range-etcd-disk-fsync-db-duration-p99.json.gz`
- `query_range-etcd-peer-round-trip-time.json.gz`
- `query_range-etcd-total-leader-elections-day.json.gz`
- `query_range-etcd-request-duration-p99.json.gz`
- `query_range-api-kas-request-duration-p99.json.gz`

### Common Pitfalls

- **Go template conflict**: Use `[[` / `]]` for Go template delimiters, not `{{` / `}}` (conflicts with Vue)
- **`v-html` scripts don't execute**: `<script>` tags injected via `v-html` are ignored. Use `$nextTick()` callbacks instead
- **Chart.js needs `<canvas>`**, not `<div>` (unlike Plotly which uses `<div>`)
- **Plotly is still used** by `metrics/index.html` (standalone page). Don't remove the Plotly JSON generation in the Go backend
- **`hasMetricsData` flag**: Gate chart rendering on `this.report.summary.features.hasMetricsData` — it's `false` when must-gather metrics were not collected

### Chatbot (AI Assistant) Architecture

> **Experimental:** The chat widget and `internal/chat/` backend are in development — see [PR #213](https://github.com/redhat-openshift-ecosystem/opct/pull/213). The guidance below applies when that feature is merged or checked out locally.

The report includes a floating chat widget powered by Claude via the Anthropic SDK.

**Backend files** (`internal/chat/`):
- `handler.go` — HTTP handlers, SSE streaming, Claude API tool-use loop
- `tools.go` — 8 tool definitions (summary, checks, plugin results, test failure logs, etcd, network) + tool executor reading from report directory
- `session.go` — Session CRUD with JSON persistence in `<report-dir>/chat-sessions/`
- `prompt.go` — Built-in system prompt with file override (`<report-dir>/system.prompt.txt`)

**API endpoints** (registered in `pkg/cmd/report/report.go`):
- `GET /api/v1/chat/status` — Returns `{"enabled": bool, "provider": "vertex"|"anthropic", "model": "..."}`
- `POST /api/v1/chat` — Send message, stream response via SSE
- `GET /api/v1/chat/sessions` — List saved sessions
- `GET /api/v1/chat/sessions/{id}` — Load a session
- `POST /api/v1/chat/sessions` — Save a session

**Authentication** (auto-detected at startup):
1. Vertex AI: detected when `GOOGLE_CLOUD_LOCATION` (or `CLOUD_ML_REGION`) and `ANTHROPIC_VERTEX_PROJECT_ID` (or `GOOGLE_CLOUD_PROJECT`) are set
2. Anthropic API: detected when `ANTHROPIC_API_KEY` is set
3. Neither set → chat disabled, UI shows setup instructions

**Key design decisions**:
- Uses **tool use** instead of stuffing the 2MB report JSON into context — Claude calls tools to fetch specific data on demand
- **SSE streaming** for real-time token display via `fetch()` + `ReadableStream`
- **AbortController** for the Stop button — cancels the fetch request mid-stream
- Tool results are read directly from the report directory filesystem

**Vertex AI region priority**: Use `GOOGLE_CLOUD_LOCATION` first, then `CLOUD_ML_REGION`. The `CLOUD_ML_REGION=global` value is NOT a valid Vertex AI Anthropic endpoint — always prefer the specific region.

**Model format**: Use alias form like `claude-sonnet-4-5` (not dated versions like `claude-sonnet-4-5-20250514` which may not be available in all Vertex projects).

### Floating Widget Pattern

The chat uses a floating widget (not a tab) so users can keep navigating all existing tabs while the chat is open.

**Key behaviors**:
- Chat bubble icon fixed bottom-right, always visible
- Three header buttons: minimize (`—`), maximize (`□`), close (`×`)
- Minimize/maximize use CSS classes (`chat-minimized`, `chat-maximized`) with `!important` rules — **never** toggle `display` on individual children via JS (causes partial state bugs)
- Chat state persists in JS variables across tab navigation
- Sessions auto-save after each assistant response

**Anti-patterns learned**:
- ❌ CSS `resize: both` — unreliable across browsers, resize handle is invisible/hard to find
- ❌ Setting `display` on individual children for minimize — causes blank areas on restore
- ❌ `direction: rtl` hack for resize handle position — conflicts with content layout
- ✅ Use CSS classes with `!important` for state changes (minimize/maximize)
- ✅ Use explicit maximize/restore buttons instead of drag-to-resize for floating panels

### UI Styling Patterns

**Page headline bar**: Cluster info displayed as colored badge pills (not raw text with brackets):
```javascript
// Pattern used in pageHeadline computed property
headline += '<span class="headline-badge headline-ocp">OpenShift <strong>' + version + '</strong></span>'
```
- `.headline-ocp` — blue badge for OpenShift version
- `.headline-k8s` — green badge for Kubernetes version
- `.headline-platform` — amber badge for platform type
- `.headline-archive` — gray monospace badge for archive filename on second row

**Table section headers**: Use Bootstrap `<dt>` with `text-light bg-secondary` class:
```javascript
this.menuBody += '<dt class="text-light bg-secondary ps-1 mb-1">Section Title</dt>'
```

**Font sizes**: Chat widget uses fixed pixel sizes (not rem) to avoid inheritance issues:
- Body text: 12px
- Headings inside chat: 13px
- Code/pre: 11px
- Tables: 11px

---

## Document Maintenance

This document should be updated when:
- New development patterns are established
- Common tasks are identified that need standardization
- AI assistants encounter repeated questions or issues
- Development workflows change significantly

**Last Updated**: 2026-05-14
**Maintainer**: OPCT Development Team
