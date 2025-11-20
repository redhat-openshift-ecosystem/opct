# OPCT Architecture Documentation

**Date**: 2025-11-20
**Version**: v0.6.0
**Official Documentation**: https://redhat-openshift-ecosystem.github.io/opct/

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Two-Repository Architecture](#two-repository-architecture)
3. [Image Dependency Chain](#image-dependency-chain)
4. [How Suite Names Flow Through The System](#suite-name-flow)
5. [Build and Release Process](#build-and-release-process)
6. [Template Rendering System](#template-rendering-system)
7. [Debugging Pod Failures](#debugging-pod-failures)
8. [Version Coordination](#version-coordination)

---

## Project Overview

**OPCT (OpenShift Provider Certification Tool)** is a tool for orchestrating conformance test workflows on OpenShift/OKD clusters. It validates that custom OpenShift installations meet the required conformance standards.

**Key Components:**
- Built on top of **Sonobuoy** for test environment management
- Uses **openshift-tests** utility to run conformance test suites
- Supports multiple deployment topologies (HA, Compact, Single Node)
- Designed for Red Hat Partner validation programs

**Primary Use Cases:**
- Red Hat Partner OpenShift validation
- Cluster conformance testing
- Upgrade path validation
- Disconnected/air-gapped environment testing

---

## Two-Repository Architecture

OPCT consists of two primary repositories that work together:

### 1. OPCT Repository
**Location**: `/home/jcallen/Development/opct`
**GitHub**: https://github.com/redhat-openshift-ecosystem/opct

**Purpose**: Command-line interface and orchestration

**Responsibilities:**
- CLI binary (`opct`) - runs on user's machine or bastion host
- Plugin manifest templates (embedded in binary)
- Test orchestration logic
- Version detection and suite selection
- Result processing and reporting

**Builds:**
- CLI binary: `opct` (Linux, macOS, Windows)
- Container image: `quay.io/opct/opct:v0.6.0`

**Key Files:**
```
opct/
├── pkg/
│   ├── types.go                    # Image version constants
│   ├── run/
│   │   ├── run.go                  # Orchestration logic, version detection
│   │   └── manifests.go            # Template rendering
├── data/
│   └── templates/
│       └── plugins/                # Plugin pod manifests (embedded in binary)
│           ├── openshift-kube-conformance.yaml
│           ├── openshift-conformance-validated.yaml
│           ├── openshift-cluster-upgrade.yaml
│           ├── openshift-conformance-replay.yaml
│           └── openshift-artifacts-collector.yaml
├── hack/
│   └── Containerfile               # Builds CLI container image
└── Makefile                        # Build targets
```

### 2. Provider-Certification-Plugins Repository
**Location**: `/home/jcallen/Development/provider-certification-plugins`
**GitHub**: https://github.com/redhat-openshift-ecosystem/provider-certification-plugins

**Purpose**: Plugin container images with test execution logic

**Responsibilities:**
- Test execution scripts (`entrypoint-tests.sh`)
- Plugin orchestration logic
- Artifact collection
- Must-gather monitoring

**Builds:**
- `quay.io/opct/plugin-openshift-tests:v0.6.0`
- `quay.io/opct/plugin-artifacts-collector:v0.6.0`
- `quay.io/opct/must-gather-monitoring:v0.6.0`
- `quay.io/opct/tools:latest` (base utility image)

**Key Files:**
```
provider-certification-plugins/
├── openshift-tests-provider-cert/
│   ├── Containerfile               # Builds plugin-openshift-tests image
│   ├── entrypoint-tests.sh         # Main test execution script
│   └── platform.sh                 # Platform-specific setup
├── artifacts-collector/
│   └── Containerfile               # Builds collector image
├── must-gather-monitoring/
│   └── Containerfile               # Builds must-gather image
└── .github/
    └── workflows/
        └── ci.yaml                 # Auto-builds images on tag/push
```

---

## Image Dependency Chain

### Complete Flow Diagram

```
User: opct run
    |
    v
┌─────────────────────────────────────────────────────────────┐
│ OPCT CLI (quay.io/opct/opct:v0.6.0)                         │
│ - Detects OCP version                                        │
│ - Sets KubeConformanceSuiteName                             │
│ - Renders plugin templates                                   │
│ - Creates Sonobuoy resources                                │
└─────────────────────────────────────────────────────────────┘
    |
    v
┌─────────────────────────────────────────────────────────────┐
│ Sonobuoy Aggregator Pod                                      │
│ Image: quay.io/opct/sonobuoy:v0.57.3                        │
│ - Orchestrates plugin execution                              │
│ - Collects results                                           │
└─────────────────────────────────────────────────────────────┘
    |
    +---> Creates Multiple Plugin Pods:
    |
    |
    v
┌─────────────────────────────────────────────────────────────┐
│ Plugin Pod: 10-openshift-kube-conformance                   │
│                                                               │
│ Init Container 1: sync                                       │
│   Image: quay.io/opct/plugin-openshift-tests:v0.6.0         │
│   Command: cp entrypoint-tests.sh → /tmp/shared/            │
│                                                               │
│ Init Container 2: login                                      │
│   Image: image-registry.openshift.../openshift/tests        │
│   Command: oc login (creates kubeconfig)                    │
│                                                               │
│ Container: tests                                             │
│   Image: image-registry.openshift.../openshift/tests        │
│   Command: /bin/bash /tmp/shared/entrypoint-tests.sh        │
│   Env: DEFAULT_SUITE_NAME=kubernetes/conformance/parallel   │
│        (OCP 4.20+) or kubernetes/conformance (< 4.20)       │
│   Runs: openshift-tests run $DEFAULT_SUITE_NAME             │
│                                                               │
│ Sidecar Container: plugin                                   │
│   Image: quay.io/opct/plugin-openshift-tests:v0.6.0         │
│   Command: openshift-tests-plugin run                        │
│   Monitors test execution, uploads results                   │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Plugin Pod: 20-openshift-conformance-validated              │
│ (Same structure as above)                                    │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Plugin Pod: 80-openshift-tests-replay                       │
│ (Same structure as above)                                    │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Plugin Pod: 05-openshift-cluster-upgrade                    │
│ (Used when --mode upgrade)                                   │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Plugin Pod: 99-openshift-artifacts-collector                │
│   Image: quay.io/opct/plugin-artifacts-collector:v0.6.0     │
│   Uses: quay.io/opct/must-gather-monitoring:v0.6.0          │
│   Collects must-gather, metrics, logs                        │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Dedicated E2E Controller Deployment                          │
│   Image: quay.io/opct/opct:v0.6.0                           │
│   Command: opct adm e2e-dedicated controller                 │
│   Manages test node resources                                │
└─────────────────────────────────────────────────────────────┘
```

### Image Reference Details (pkg/types.go)

```go
const (
    DefaultToolsRepository         = "quay.io/opct"
    ControllerImage                = "quay.io/opct/opct:v0.6.0"

    // Plugin images - NO registry prefix (added dynamically)
    PluginsImage                   = "plugin-openshift-tests:v0.6.0"
    CollectorImage                 = "plugin-artifacts-collector:v0.6.0"
    MustGatherMonitoringImage      = "must-gather-monitoring:v0.6.0"

    // OpenShift tests from cluster's internal registry
    OpenShiftTestsImage            = "image-registry.openshift-image-registry.svc:5000/openshift/tests"
)

// Registry prefix added at runtime:
func GetPluginsImage() string {
    return fmt.Sprintf("%s/%s", DefaultToolsRepository, PluginsImage)
    // Returns: "quay.io/opct/plugin-openshift-tests:v0.6.0"
}
```

**Critical Detail**: Plugin image names don't include the registry in the constants. The registry is prepended at runtime by helper functions.

---

## Suite Name Flow Through The System

### The Complete Journey (OCP 4.20+ Example)

```
┌─────────────────────────────────────────────────────────────┐
│ 1. User Executes Command                                     │
│    $ opct run --watch                                        │
└─────────────────────────────────────────────────────────────┘
                        |
                        v
┌─────────────────────────────────────────────────────────────┐
│ 2. PreRunCheck() - pkg/run/run.go                           │
│    - Creates ConfigV1 client                                 │
│    - Calls setKubeConformanceSuiteName(oc)                  │
│      - Fetches ClusterVersion from API                       │
│      - Parses version: "4.20.0" → major=4, minor=20         │
│      - Checks: if major==4 && minor>=20                      │
│      - Sets: r.KubeConformanceSuiteName =                   │
│              "kubernetes/conformance/parallel"               │
│      - Logs: "Using kubernetes/conformance/parallel          │
│               suite for OCP 4.20"                            │
└─────────────────────────────────────────────────────────────┘
                        |
                        v
┌─────────────────────────────────────────────────────────────┐
│ 3. Run() - pkg/run/run.go                                   │
│    - Calls loadPluginManifests(r)                           │
│      - Reads embedded template files from binary            │
│      - Processes each *.yaml in data/templates/plugins/     │
│      - Calls ProcessManifestTemplates(r, manifest)          │
└─────────────────────────────────────────────────────────────┘
                        |
                        v
┌─────────────────────────────────────────────────────────────┐
│ 4. Template Rendering - pkg/run/manifests.go                │
│    Input Template:                                           │
│      env:                                                    │
│        - name: DEFAULT_SUITE_NAME                           │
│          value: "{{ .KubeConformanceSuiteName }}"           │
│                                                              │
│    Go Template Engine replaces variables:                   │
│      {{ .KubeConformanceSuiteName }} →                      │
│         "kubernetes/conformance/parallel"                    │
│                                                              │
│    Rendered YAML:                                            │
│      env:                                                    │
│        - name: DEFAULT_SUITE_NAME                           │
│          value: "kubernetes/conformance/parallel"           │
└─────────────────────────────────────────────────────────────┘
                        |
                        v
┌─────────────────────────────────────────────────────────────┐
│ 5. Sonobuoy Creates Plugin Pod                              │
│    Pod spec includes rendered environment variable:         │
│      containers:                                             │
│        - name: tests                                         │
│          env:                                                │
│            - name: DEFAULT_SUITE_NAME                       │
│              value: "kubernetes/conformance/parallel"       │
└─────────────────────────────────────────────────────────────┘
                        |
                        v
┌─────────────────────────────────────────────────────────────┐
│ 6. Init Container Executes                                   │
│    Image: quay.io/opct/plugin-openshift-tests:v0.6.0        │
│    Command: cp entrypoint-tests.sh → /tmp/shared/           │
│    (Copies script from plugin image to shared volume)       │
└─────────────────────────────────────────────────────────────┘
                        |
                        v
┌─────────────────────────────────────────────────────────────┐
│ 7. Test Container Executes                                   │
│    Image: openshift/tests (from cluster registry)           │
│    Command: /bin/bash /tmp/shared/entrypoint-tests.sh       │
│                                                              │
│    Script reads environment:                                 │
│      SUITE_NAME=${SUITE_NAME:-${DEFAULT_SUITE_NAME-}}      │
│      # SUITE_NAME = "kubernetes/conformance/parallel"       │
│                                                              │
│    Executes:                                                 │
│      openshift-tests run kubernetes/conformance/parallel \  │
│        --dry-run -o /tmp/suite-list.txt                     │
│      openshift-tests run kubernetes/conformance/parallel    │
└─────────────────────────────────────────────────────────────┘
```

### Key Points:

1. **Version Detection**: Happens ONCE at CLI startup in PreRunCheck()
2. **Template Variables**: Embedded templates use `{{ .FieldName }}` syntax
3. **No Hardcoding**: Suite names are never hardcoded in rendered manifests
4. **Script Independence**: entrypoint-tests.sh doesn't know about versions - it just reads `$DEFAULT_SUITE_NAME`
5. **Backward Compatibility**: Works with any version of plugin images because interface is environment variables

---

## Build and Release Process

### Development Workflow

#### Building OPCT CLI Locally
```bash
cd /home/jcallen/Development/opct
make build                  # Builds for current OS/arch
make build-linux-amd64      # Builds Linux binary
make linux-amd64-container  # Builds container image
```

#### Building Plugin Images Locally
```bash
cd /home/jcallen/Development/provider-certification-plugins
make images                 # Builds all plugin images
make image-openshift-tests  # Builds just the test plugin
```

### Production Release Process

**Step 1: Release Plugin Images**

```bash
cd provider-certification-plugins
git tag v0.6.0
git push origin v0.6.0
```

- GitHub Actions workflow (`.github/workflows/ci.yaml`) triggers
- Builds multi-arch images (amd64, arm64)
- Publishes to quay.io/opct/:
  - `plugin-openshift-tests:v0.6.0`
  - `plugin-artifacts-collector:v0.6.0`
  - `must-gather-monitoring:v0.6.0`

**Step 2: Update OPCT Repository**

```bash
cd opct
# Update pkg/types.go with new plugin versions
vim pkg/types.go
# Change:
#   PluginsImage = "plugin-openshift-tests:v0.6.0"
#   CollectorImage = "plugin-artifacts-collector:v0.6.0"
#   etc.

git add pkg/types.go
git commit -m "Update plugin images to v0.6.0"
git push origin main
```

**Step 3: Release OPCT CLI**

```bash
git tag v0.6.0
git push origin v0.6.0
```

- GitHub Actions builds CLI binaries (Linux, macOS, Windows)
- Builds container image
- Publishes to:
  - GitHub Releases (binaries)
  - `quay.io/opct/opct:v0.6.0` (container)

### Version Alignment Strategy

**Critical Requirements:**
1. Plugin image versions must exist BEFORE updating OPCT
2. Both repos should use compatible Sonobuoy library versions
3. Test compatibility between CLI and plugin versions

**Version Compatibility Matrix:**

| OPCT Version | Plugin Version | Sonobuoy Version | OCP Versions Supported |
|--------------|----------------|------------------|------------------------|
| v0.6.0       | v0.6.0         | v0.57.3          | 4.14-4.21              |
| v0.5.0       | v0.5.0         | v0.57.x          | 4.12-4.18              |

---

## Template Rendering System

### How Templates Are Embedded

During OPCT CLI build:
1. Go's `embed` package includes `data/templates/` in binary
2. Templates are read from embedded filesystem at runtime
3. No external files needed - binary is self-contained

**Code Reference (internal/assets/assets.go):**
```go
//go:embed data/templates/*
var dataFS embed.FS

func GetData() embed.FS {
    return dataFS
}
```

### Template Variables Available

**RunOptions struct fields become template variables:**

```go
type RunOptions struct {
    PluginsImage              string  // → {{ .PluginsImage }}
    CollectorImage            string  // → {{ .CollectorImage }}
    MustGatherMonitoringImage string  // → {{ .MustGatherMonitoringImage }}
    OpenshiftTestsImage       string  // → {{ .OpenshiftTestsImage }}
    KubeConformanceSuiteName  string  // → {{ .KubeConformanceSuiteName }}
    // ... other fields
}
```

### Template Usage Example

**Template (data/templates/plugins/openshift-kube-conformance.yaml):**
```yaml
podSpec:
  initContainers:
    - name: sync
      image: "{{ .PluginsImage }}"
      imagePullPolicy: Always
  containers:
    - name: tests
      image: "{{ .OpenshiftTestsImage }}"
      env:
        - name: DEFAULT_SUITE_NAME
          value: "{{ .KubeConformanceSuiteName }}"
```

**Rendered for OCP 4.20:**
```yaml
podSpec:
  initContainers:
    - name: sync
      image: "quay.io/opct/plugin-openshift-tests:v0.6.0"
      imagePullPolicy: Always
  containers:
    - name: tests
      image: "image-registry.openshift-image-registry.svc:5000/openshift/tests"
      env:
        - name: DEFAULT_SUITE_NAME
          value: "kubernetes/conformance/parallel"
```

### Template Processing Flow

```
User runs: opct run
    ↓
pkg/run/run.go: Run()
    ↓
pkg/run/manifests.go: loadPluginManifests(RunOptions)
    ↓
For each *.yaml in data/templates/plugins/:
    ↓
    Read template from embedded FS
    ↓
    pkg/run/manifests.go: ProcessManifestTemplates()
        ↓
        template.New("manifest").Parse(yamlContent)
        ↓
        template.Execute(&buffer, runOptions)
        ↓
        Returns rendered YAML
    ↓
Sonobuoy loads rendered manifests
    ↓
Creates Kubernetes resources
```

---

## Debugging Pod Failures

### Common Issues and Diagnosis

#### 1. ImagePullBackOff / ErrImagePull

**Symptoms:**
```
$ kubectl get pods -n opct
NAME                                        READY   STATUS             RESTARTS   AGE
sonobuoy-10-openshift-kube-conformance-...  0/3     ImagePullBackOff   0          2m
```

**Diagnosis:**
```bash
kubectl describe pod <pod-name> -n opct
# Look for:
#   Events:
#     Normal   BackOff    2m    kubelet  Back-off pulling image "quay.io/opct/plugin-openshift-tests:v0.6.0"
#     Warning  Failed     2m    kubelet  Failed to pull image "quay.io/opct/plugin-openshift-tests:v0.6.0": rpc error: code = NotFound
```

**Common Causes:**
- Image tag doesn't exist in registry
- Image is private and no pull secret configured
- Registry is unreachable (network/firewall)
- Typo in image name or tag

**Resolution:**
```bash
# Verify image exists
podman pull quay.io/opct/plugin-openshift-tests:v0.6.0

# Check if image is public
curl -s https://quay.io/api/v1/repository/opct/plugin-openshift-tests

# For disconnected environments, mirror images:
oc image mirror \
  quay.io/opct/plugin-openshift-tests:v0.6.0 \
  ${MIRROR_REGISTRY}/opct/plugin-openshift-tests:v0.6.0
```

#### 2. CrashLoopBackOff - Suite Name Error

**Symptoms:**
```
$ kubectl get pods -n opct
NAME                                        READY   STATUS             RESTARTS   AGE
sonobuoy-10-openshift-kube-conformance-...  2/3     CrashLoopBackOff   5          5m
```

**Diagnosis:**
```bash
kubectl logs <pod-name> -n opct -c tests
# Look for:
#   error: error converting to options: suite "kubernetes/conformance" does not exist
```

**Cause**: Suite name mismatch (the error this fix addresses!)

**Resolution**: Use OPCT version with OCP 4.20+ fix applied

#### 3. OpenShift Tests Image Not Found

**Symptoms:**
```
Error pulling image: image-registry.openshift-image-registry.svc:5000/openshift/tests:latest not found
```

**Cause**: Tests image not imported to cluster's internal registry

**Resolution:**
```bash
# Check if image exists in cluster
oc get imagestream tests -n openshift

# If missing, it should be auto-imported, but can manually import:
oc import-image tests --from=quay.io/openshift/origin-tests:4.20 \
  --confirm --scheduled -n openshift
```

#### 4. Permission Errors

**Symptoms:**
```
Error creating namespace: User "..." cannot create resource "namespaces"
```

**Cause**: Insufficient RBAC permissions

**Resolution:**
```bash
# Verify you're cluster-admin
oc whoami
oc auth can-i create namespaces

# If not cluster-admin, cannot run OPCT
```

### Debugging Commands Cheatsheet

```bash
# Check pod status
kubectl get pods -n opct
kubectl get pods -n opct -o wide

# Describe pod (shows events, image pulls)
kubectl describe pod <pod-name> -n opct

# View logs from all containers
kubectl logs <pod-name> -n opct --all-containers=true

# View logs from specific container
kubectl logs <pod-name> -n opct -c tests
kubectl logs <pod-name> -n opct -c plugin
kubectl logs <pod-name> -n opct -c sync  # init container

# Check previous container logs (after restart)
kubectl logs <pod-name> -n opct -c tests --previous

# Get events
kubectl get events -n opct --sort-by='.lastTimestamp'

# Check Sonobuoy aggregator
kubectl logs -n opct sonobuoy

# Check if images are pullable
kubectl run test-pull --rm -it --restart=Never \
  --image=quay.io/opct/plugin-openshift-tests:v0.6.0 \
  -- /bin/sh

# Verify suite names in running pod
kubectl exec <pod-name> -n opct -c tests -- env | grep SUITE
```

---

## Version Coordination

### Why Two Repositories?

**Separation of Concerns:**
1. **OPCT (opct)**: Orchestration logic, workflow, CLI UX
   - Changes: New features, bug fixes, workflow improvements
   - Release frequency: As needed for feature/fix releases

2. **Plugins (provider-certification-plugins)**: Test execution runtime
   - Changes: Test suite updates, platform support, entrypoint logic
   - Release frequency: When tests change or platform support added

**Benefits:**
- Plugin images can be updated independently
- CLI can reference different plugin versions (via pkg/types.go)
- Backward compatibility easier to maintain
- Different teams can maintain different repos

### Coordination Points

**When Coordination IS Required:**

1. **Breaking Changes to Interface**
   - Changing environment variable names
   - Modifying shared volume structure
   - Altering result format

2. **Major Releases**
   - Both repos should align on version numbers
   - Both should reference compatible Sonobuoy versions

3. **New Plugin Types**
   - New plugin templates in OPCT
   - New plugin images in provider-certification-plugins
   - Both must be released together

**When Coordination NOT Required:**

1. **Suite Name Logic (This Fix!)**
   - OPCT determines suite name
   - Passes via environment variable
   - Plugin reads variable - no changes needed

2. **Bug Fixes in Either Repo**
   - OPCT CLI bug: Fix and release
   - Plugin script bug: Fix plugin, update OPCT reference

3. **Documentation Updates**
   - Each repo maintains own docs

### Version Update Checklist

When updating plugin versions in OPCT:

```bash
# 1. Verify plugin images exist
podman pull quay.io/opct/plugin-openshift-tests:v0.7.0
podman pull quay.io/opct/plugin-artifacts-collector:v0.7.0
podman pull quay.io/opct/must-gather-monitoring:v0.7.0

# 2. Update pkg/types.go
vim pkg/types.go
# Update version constants

# 3. Test locally
make build
./opct run --watch

# 4. Commit and tag
git add pkg/types.go
git commit -m "Update plugins to v0.7.0"
git tag v0.7.0
git push origin main --tags
```

---

## Additional Resources

- **Official Documentation**: https://redhat-openshift-ecosystem.github.io/opct/
- **OPCT GitHub**: https://github.com/redhat-openshift-ecosystem/opct
- **Plugins GitHub**: https://github.com/redhat-openshift-ecosystem/provider-certification-plugins
- **Sonobuoy**: https://sonobuoy.io/
- **OpenShift Tests**: Part of OpenShift/Origin repository

---

## Appendix: File Locations Quick Reference

### OPCT Repository
```
/home/jcallen/Development/opct/
├── pkg/types.go                           # Image version constants
├── pkg/run/run.go                         # Version detection, suite selection
├── pkg/run/manifests.go                   # Template rendering
├── data/templates/plugins/*.yaml          # Plugin manifests (embedded)
├── hack/Containerfile                     # CLI container build
├── Makefile                               # Build targets
└── CHANGES_OCP420_SUITE_FIX.md           # OCP 4.20+ suite fix details
```

### Provider-Certification-Plugins Repository
```
/home/jcallen/Development/provider-certification-plugins/
├── openshift-tests-provider-cert/
│   ├── Containerfile                      # plugin-openshift-tests image
│   ├── entrypoint-tests.sh               # Test execution script
│   └── platform.sh                        # Platform setup
├── artifacts-collector/Containerfile      # Collector image
├── must-gather-monitoring/Containerfile   # Must-gather image
└── .github/workflows/ci.yaml             # Auto-build workflow
```

---

**Document Version**: 1.0
**Last Updated**: 2025-11-20
**Maintainer**: OPCT Development Team
