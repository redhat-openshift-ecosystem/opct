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
  - [OPCT CLI Release](#opct-cli-release)
  - [Plugins Release](#plugins-release)
- [Validation Procedures](#validation-procedures)
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

**Reference**: [docs/devel/release.md](docs/devel/release.md) and [docs/devel/update.md](docs/devel/update.md)

### Release Overview

OPCT project consists of two main repositories that need to be released independently but in coordination:

1. **OPCT CLI** ([github.com/redhat-openshift-ecosystem/opct](https://github.com/redhat-openshift-ecosystem/opct))
   - Client-side CLI tool
   - Delivered as binary and container image
   - Repository: `quay.io/opct/opct`

2. **OPCT Plugins** ([github.com/redhat-openshift-ecosystem/provider-certification-plugins](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins))
   - Container-based workflow steps
   - Plugin images: openshift-tests, artifacts-collector, must-gather-monitoring, tools
   - Repository: `quay.io/opct/plugin-*`

**Release branches**:
- CLI: `release-X.Y` (e.g., `release-0.6`)
- Plugins: `release-vX.Y` (e.g., `release-v0.6`)

**Version format**: `vX.Y.Z` (e.g., `v0.6.1`)

### OPCT CLI Release

**Goal**: Release a new patch version (e.g., `v0.6.1`) of the OPCT CLI.

#### Prerequisites

- Changes are merged to `main` branch
- Local `main` branch is up to date
- You have write access to the repository

#### Step-by-Step Process

**1. Update main branch with latest changes**

```bash
git checkout main
git pull origin main
```

**2. Create version bump PR**

Update plugin image versions in `pkg/types.go`:

```go
// pkg/types.go (lines 23-25)
PluginsImage              = "plugin-openshift-tests:v0.6.1"      // Change v0.6.0 → v0.6.1
CollectorImage            = "plugin-artifacts-collector:v0.6.1"  // Change v0.6.0 → v0.6.1
MustGatherMonitoringImage = "must-gather-monitoring:v0.6.1"      // Change v0.6.0 → v0.6.1
```

**Why update pkg/types.go?**
- The CLI references specific plugin image versions
- This PR tests that the new version works correctly in CI
- Merging to `main` ensures the release tag will reference correct plugin versions

**Create and push PR**:

```bash
# Create feature branch
git checkout -b release/bump-v0.6.1

# Edit pkg/types.go
# Update the three image version strings

# Commit changes
git add pkg/types.go
git commit -m "chore: bump version to v0.6.1

Prepare for v0.6.1 release by updating plugin image versions
in pkg/types.go. This ensures the CLI will reference the correct
plugin images when v0.6.1 is released.

Changes:
- PluginsImage: v0.6.0 → v0.6.1
- CollectorImage: v0.6.0 → v0.6.1
- MustGatherMonitoringImage: v0.6.0 → v0.6.1

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

# Push and create PR
git push origin release/bump-v0.6.1
# Create PR via GitHub UI or gh CLI
```

**3. Wait for PR review and merge**

- Ensure all CI checks pass
- Request review from maintainers
- Wait for approval and merge to `main`

**4. Update main branch after merge**

```bash
git checkout main
git pull origin main
```

**5. Update release branch from main**

```bash
# Checkout the release branch (e.g., release-0.6 for v0.6.x releases)
git checkout release-0.6
git pull origin release-0.6

# Rebase from main to include latest changes
git rebase main

# Push updated release branch
git push origin release-0.6
```

**6. Create and push release tag**

```bash
# Ensure you're on the updated release branch
git checkout release-0.6

# Verify the branch includes the version bump
git log --oneline -5
# Should show the version bump commit

# Create annotated tag
git tag -a v0.6.1 -m "Release v0.6.1

This release includes:
- [Brief description of changes]
- [Bug fixes]
- [New features]

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

# Push tag to origin
git push origin v0.6.1
```

**7. CI automatically builds and publishes**

Once the tag is pushed:
- GitHub Actions workflow `.github/workflows/ci.yaml` triggers
- Builds are created for `linux/amd64` and `linux/arm64`
- Images are pushed to `quay.io/opct/opct:v0.6.1`
- Binaries are attached to GitHub release

**8. Verify release**

```bash
# Check image was published
skopeo list-tags docker://quay.io/opct/opct | grep v0.6.1

# Check GitHub release
# Visit: https://github.com/redhat-openshift-ecosystem/opct/releases/tag/v0.6.1
```

#### Release Branch Strategy

**When to create a new release branch**:
- For new minor versions (e.g., `v0.7.0` → create `release-0.7`)
- Release branches track major.minor versions (e.g., `release-0.6` for all `v0.6.x`)

**For patch releases**:
- Use existing release branch (e.g., `release-0.6` for `v0.6.1`, `v0.6.2`, etc.)
- Rebase from `main` to pick up latest changes
- Create new tag from updated release branch

### Plugins Release

**Goal**: Release new plugin container images (e.g., `v0.6.1`).

**Repository**: [github.com/redhat-openshift-ecosystem/provider-certification-plugins](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins)

#### Release Process

The plugin release process is **identical** to the CLI release process:

**1. Update main branch**

```bash
git checkout main
git pull origin main
```

**2. Update release branch from main**

```bash
# Checkout the release branch (e.g., release-v0.6 for v0.6.x releases)
git checkout release-v0.6
git pull origin release-v0.6

# Rebase from main to include latest changes
git rebase main

# Push updated release branch
git push origin release-v0.6
```

**3. Create and push release tag**

```bash
# Ensure you're on the updated release branch
git checkout release-v0.6

# Create annotated tag
git tag -a v0.6.1 -m "Release v0.6.1

This release includes:
- [Brief description of plugin changes]
- [Bug fixes]
- [New features]

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

# Push tag to origin
git push origin v0.6.1
```

**4. CI automatically builds and publishes**

Once the tag is pushed:
- GitHub Actions workflow `.github/workflows/ci.yaml` triggers
- Builds all plugin images for `linux/amd64` and `linux/arm64`
- Images are pushed to:
  - `quay.io/opct/plugin-openshift-tests:v0.6.1`
  - `quay.io/opct/plugin-artifacts-collector:v0.6.1`
  - `quay.io/opct/must-gather-monitoring:v0.6.1`
  - `quay.io/opct/tools:v0.6.1`

**5. Verify release**

```bash
# Check images were published
skopeo list-tags docker://quay.io/opct/plugin-openshift-tests | grep v0.6.1
skopeo list-tags docker://quay.io/opct/plugin-artifacts-collector | grep v0.6.1
skopeo list-tags docker://quay.io/opct/must-gather-monitoring | grep v0.6.1
```

#### Key Differences from CLI Release

- **No version bump PR needed**: Plugins don't have a central version file like CLI's `pkg/types.go`
- **Release branch naming**: Uses `release-vX.Y` format (with `v` prefix)
- **Multiple images**: Single tag publishes 4 different container images
- **Must be released BEFORE CLI**: CLI references plugin image versions in `pkg/types.go`

### Release Coordination

**Recommended release order**:

1. **Plugins first**: Release plugins with new tag (e.g., `v0.6.1`)
2. **Verify plugins**: Ensure all plugin images are published
3. **CLI second**: Update `pkg/types.go` to reference new plugin versions, then release CLI

**Why this order?**
- CLI references specific plugin image versions in `pkg/types.go`
- If CLI is released first, it would reference plugin versions that don't exist yet
- This order ensures all referenced images are available

### Common Release Scenarios

#### Scenario 1: Patch release with bug fixes in both CLI and plugins

```bash
# 1. Release plugins
cd provider-certification-plugins
git checkout main && git pull
git checkout release-v0.6 && git rebase main && git push
git tag -a v0.6.1 -m "Release v0.6.1" && git push origin v0.6.1

# 2. Wait for plugin images to build (check CI)

# 3. Release CLI
cd opct
git checkout -b release/bump-v0.6.1
# Edit pkg/types.go to reference v0.6.1 plugin images
git commit -m "chore: bump version to v0.6.1"
git push origin release/bump-v0.6.1
# Create PR, get reviewed, merge

# 4. After PR merged
git checkout main && git pull
git checkout release-0.6 && git rebase main && git push
git tag -a v0.6.1 -m "Release v0.6.1" && git push origin v0.6.1
```

#### Scenario 2: CLI-only changes (no plugin changes)

```bash
# 1. No plugin release needed

# 2. Release CLI (referencing existing plugin version, e.g., v0.6.0)
cd opct
git checkout -b release/bump-v0.6.1
# Edit pkg/types.go - plugin versions may stay at v0.6.0
# Only update if you want to ensure latest plugins are used
git commit -m "chore: bump version to v0.6.1"
# Continue with normal CLI release process
```

#### Scenario 3: New minor version (v0.7.0)

```bash
# 1. Create new release branches
cd provider-certification-plugins
git checkout main
git checkout -b release-v0.7
git push origin release-v0.7

cd opct
git checkout main
git checkout -b release-0.7
git push origin release-0.7

# 2. Follow normal release process with new branches
```

### Release Checklist

**Pre-release**:
- [ ] All changes merged to `main` in both repositories
- [ ] CI passing on `main` branch
- [ ] Version numbers decided (e.g., `v0.6.1`)

**Plugins release**:
- [ ] Update `main` branch: `git checkout main && git pull`
- [ ] Update release branch: `git checkout release-v0.6 && git pull origin release-v0.6`
- [ ] Rebase from main: `git rebase main`
- [ ] Push release branch: `git push origin release-v0.6`
- [ ] Create annotated tag with comprehensive changelog (see tag creation example above)
- [ ] Push tag: `git push origin v0.6.1`
- [ ] Monitor CI build: `gh run watch`
- [ ] Verify images in registry: `skopeo list-tags docker://quay.io/opct/plugin-openshift-tests | grep v0.6.1`

**CLI release**:
- [ ] Create version bump PR updating `pkg/types.go` with new plugin versions
- [ ] Get PR reviewed and merged to `main`
- [ ] Update `main` branch: `git checkout main && git pull`
- [ ] Update release branch: `git checkout release-0.6 && git pull origin release-0.6`
- [ ] Rebase from main: `git rebase main`
- [ ] Push release branch: `git push origin release-0.6`
- [ ] Create annotated tag with comprehensive changelog (see tag creation example above)
- [ ] Push tag: `git push origin v0.6.1`
- [ ] Monitor CI build: `gh run watch`
- [ ] Verify CLI image in registry: `skopeo list-tags docker://quay.io/opct/opct | grep v0.6.1`
- [ ] Verify GitHub release created with binaries

### Lessons Learned from v0.6.1 Release

**Date**: 2025-12-06

#### What Went Wrong

1. **Automated tag creation workflow was unreliable**
   - `.github/workflows/auto-release-tag.yaml` had multiple issues
   - Complex workflow with security guardrails that were difficult to debug
   - Failed silently when PR description didn't match expected format
   - Required specific PR format that was easy to forget

2. **golangci-lint-action@v7 broke tag builds**
   - Upgrade from v6 to v7 introduced breaking change with `only-new-issues` flag
   - Flag fails on tag builds (no base commit to compare against)
   - Error: `failed to fetch push patch: RequestError [HttpError]: Not Found`
   - Blocked release for several hours while troubleshooting

3. **Over-engineering solutions**
   - Attempted conditional `only-new-issues` logic: `${{ !startsWith(github.ref, 'refs/tags/') }}`
   - Created multiple PRs trying to fix the same issue
   - Added complexity instead of simplifying

4. **Force-pushing to release branches caused confusion**
   - Multiple force-pushes to `release-0.6` during troubleshooting
   - Lost track of which commits were in which branch
   - Made it harder to understand the actual state

#### What Worked

1. **Manual tag creation is simple and reliable**
   - Direct `git tag -a` with comprehensive changelog
   - No complex workflows to debug
   - Immediate feedback if something fails

2. **Rebase workflow for release branches**
   - `git rebase main` on release branches works well
   - Keeps release branch clean and up-to-date
   - Easy to understand what changes are being released

3. **Plugin images released successfully**
   - Plugins v0.6.1 built and published without issues
   - Simpler workflow, fewer dependencies

#### Key Takeaways

1. **Keep it simple**: Manual processes are better than complex automation for infrequent tasks (releases happen ~monthly)
2. **Don't change what works**: The v0.6.0 release process worked fine with manual tags
3. **Test workflows thoroughly before relying on them**: Automated workflows need extensive testing
4. **Understand root causes before implementing fixes**: golangci-lint-action version upgrade was the real issue
5. **Use `continue-on-error` for non-critical CI steps**: Prevents one failing linter from blocking entire release

#### Recommended Approach Going Forward

**Manual tag creation** (current approach):
```bash
# 1. Update and rebase release branch
git checkout release-X.Y
git rebase main
git push origin release-X.Y

# 2. Create annotated tag with changelog
git tag -a vX.Y.Z -m "Release vX.Y.Z

[Comprehensive changelog here]

🤖 Claude Code Assistant
Co-Authored-By: Claude <noreply@anthropic.com>"

# 3. Push tag to trigger CI
git push origin vX.Y.Z
```

**Advantages**:
- ✅ Simple and predictable
- ✅ Full control over changelog content
- ✅ Easy to troubleshoot if CI fails
- ✅ No dependencies on complex workflows
- ✅ Works the same way for both CLI and Plugins

**Disadvantages**:
- ⚠️ Requires manual steps (but releases are infrequent)
- ⚠️ Requires git knowledge (but maintainers should have this)

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

### AI Assistant Footer

Include the following footer in commits made by AI assistants:

```
🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

### Branch Naming

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

## Document Maintenance

This document should be updated when:
- New development patterns are established
- Common tasks are identified that need standardization
- AI assistants encounter repeated questions or issues
- Development workflows change significantly

**Last Updated**: 2025-12-08
**Maintainer**: OPCT Development Team
