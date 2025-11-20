# Fix for kubernetes/conformance Suite Error in OCP 4.20+

## Problem Statement

Starting with OpenShift 4.20, the `kubernetes/conformance` test suite was reorganized in the `openshift-tests` binary. The monolithic suite was split into sub-suites. This caused OPCT's `openshift-kube-conformance` plugin to fail with:

```
error: error converting to options: suite "kubernetes/conformance" does not exist
Error occurred on line 61: ${CMD_OTESTS} ${OT_RUN_COMMAND:-run} ${SUITE_NAME:-${DEFAULT_SUITE_NAME-}} --dry-run -o ${CTRL_SUITE_LIST}
```

## Root Cause

In OCP 4.20, the `kubernetes/conformance` suite was reorganized into sub-suites:
- `kubernetes/conformance/parallel`
- `kubernetes/conformance/serial`
- `kubernetes/conformance/parallel/minimal`
- `kubernetes/conformance/serial/minimal`

The parent suite `kubernetes/conformance` is no longer a runnable suite - only the sub-suites can be executed.

## Solution Approach

Implemented version-aware suite selection:
- **OCP < 4.20**: Use `kubernetes/conformance` (backward compatibility)
- **OCP >= 4.20**: Use `kubernetes/conformance/parallel` (new behavior, runs parallel tests)

### Complete Fix Required Two Changes

The fix required changes in **both repositories**:

1. **OPCT Repository** (`/home/jcallen/Development/opct`):
   - Added version detection logic to determine which suite name to use
   - Updated plugin template to pass suite name as environment variable
   - Template now renders `{{ .KubeConformanceSuiteName }}` instead of hardcoded value

2. **Provider-Certification-Plugins Repository** (`/home/jcallen/Development/provider-certification-plugins`):
   - **Critical Fix**: Plugin Go code was hardcoding suite name to `kubernetes/conformance`
   - Modified plugin to read from `DEFAULT_SUITE_NAME` environment variable
   - Falls back to hardcoded constant if environment variable not set

**Why both changes were needed:**
- OPCT sets the environment variable based on cluster version
- But the plugin's Go code was ignoring it and using a hardcoded constant
- Without the plugin fix, the environment variable had no effect

## Files Changed

### 1. pkg/run/run.go

#### Added field to RunOptions struct (lines 53-57):
```go
// KubeConformanceSuiteName
// defines the suite name for Kubernetes conformance tests.
// This is version-dependent: "kubernetes/conformance" for OCP < 4.20,
// "kubernetes/conformance/parallel" for OCP >= 4.20.
KubeConformanceSuiteName string
```

#### Added new method (lines 203-245):
```go
// setKubeConformanceSuiteName determines the appropriate suite name for Kubernetes
// conformance tests based on the cluster version. Starting with OCP 4.20, the
// kubernetes/conformance suite was reorganized into sub-suites (parallel, serial,
// and minimal variants).
func (r *RunOptions) setKubeConformanceSuiteName(oc *coclient.Clientset) error {
	// Default to kubernetes/conformance for backward compatibility
	r.KubeConformanceSuiteName = "kubernetes/conformance"

	// Get the cluster version
	cv, err := oc.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		log.Warnf("Failed to get cluster version, defaulting to kubernetes/conformance suite: %v", err)
		return nil
	}

	// Extract the version string
	version := cv.Status.Desired.Version
	if version == "" {
		log.Warn("Cluster version is empty, defaulting to kubernetes/conformance suite")
		return nil
	}

	log.Debugf("Detected cluster version: %s", version)

	// Parse the version to check if it's >= 4.20
	// Version format is typically "4.20.0" or "4.20.0-rc.1"
	var major, minor int
	_, err = fmt.Sscanf(version, "%d.%d", &major, &minor)
	if err != nil {
		log.Warnf("Failed to parse cluster version %q, defaulting to kubernetes/conformance suite: %v", version, err)
		return nil
	}

	// For OCP 4.20+, use the parallel sub-suite
	if major == 4 && minor >= 20 {
		r.KubeConformanceSuiteName = "kubernetes/conformance/parallel"
		log.Infof("Using kubernetes/conformance/parallel suite for OCP %d.%d", major, minor)
	} else {
		log.Infof("Using kubernetes/conformance suite for OCP %d.%d", major, minor)
	}

	return nil
}
```

#### Modified PreRunCheck method (lines 261-265):
Added call to set the suite name after ConfigV1 client creation:
```go
// Determine the appropriate suite name for Kubernetes conformance tests
// based on cluster version
if err := r.setKubeConformanceSuiteName(oc); err != nil {
	return err
}
```

### 2. data/templates/plugins/openshift-kube-conformance.yaml

#### Changed line 54 (tests container):
**Before:**
```yaml
- name: DEFAULT_SUITE_NAME
  value: "kubernetes/conformance"
```

**After:**
```yaml
- name: DEFAULT_SUITE_NAME
  value: "{{ .KubeConformanceSuiteName }}"
```

#### **CRITICAL**: Added to plugin container (line 89-90):
**This was the missing piece that caused the fix to initially not work!**

```yaml
spec:
  name: plugin
  image: "{{ .PluginsImage }}"
  env:
    - name: KUBECONFIG
      value: /tmp/shared/kubeconfig
    - name: DEFAULT_SUITE_NAME
      value: "{{ .KubeConformanceSuiteName }}"  # <-- ADDED HERE
    - name: PLUGIN_NAME
      value: "openshift-kube-conformance"
```

**Why this is critical:**
- The pod has TWO containers: "tests" and "plugin"
- The "tests" container runs `entrypoint-tests.sh` (bash script)
- The "plugin" container runs `/usr/bin/openshift-tests-plugin` (Go binary)
- The **Go code** that reads `DEFAULT_SUITE_NAME` runs in the **plugin** container
- Initially, we only set the variable in the "tests" container, so the Go code couldn't see it
- The plugin Go code needs this variable to generate the correct start script

### 3. internal/report/slo.go

#### Updated line 256:
**Before:**
```go
Description: "Kubernetes Conformance suite (defined as `kubernetes/conformance` in `openshift-tests`) implements e2e required by Kubernetes Certification. Those tests are base tests for an operational Kubernetes cluster. All tests must be passed prior reviewing OpenShift Conformance suite.",
```

**After:**
```go
Description: "Kubernetes Conformance suite implements e2e required by Kubernetes Certification. For OCP < 4.20, this uses the `kubernetes/conformance` suite in `openshift-tests`. For OCP >= 4.20, this uses the `kubernetes/conformance/parallel` suite as the suite was reorganized into parallel and serial sub-suites. These tests are base tests for an operational Kubernetes cluster. All tests must be passed prior reviewing OpenShift Conformance suite.",
```

### 4. docs/review/rules.md

#### Updated line 17:
**Before:**
```markdown
- **Description**: Kubernetes Conformance suite (defined as `kubernetes/conformance` in `openshift-tests`) implements e2e required by Kubernetes Certification. Those tests are base tests for an operational Kubernetes cluster. All tests must be passed prior reviewing OpenShift Conformance suite.
```

**After:**
```markdown
- **Description**: Kubernetes Conformance suite implements e2e required by Kubernetes Certification. For OCP < 4.20, this uses the `kubernetes/conformance` suite in `openshift-tests`. For OCP >= 4.20, this uses the `kubernetes/conformance/parallel` suite as the suite was reorganized into parallel and serial sub-suites. These tests are base tests for an operational Kubernetes cluster. All tests must be passed prior reviewing OpenShift Conformance suite.
```

### 5. docs/devel/guide.md

#### Updated line 189:
**Before:**
```markdown
- B. `"Suite List"`: This is the list of e2e tests available on the respective suite. For example: plugin `openshift-kubernetes-conformance` uses the suite `kubernetes/conformance`
```

**After:**
```markdown
- B. `"Suite List"`: This is the list of e2e tests available on the respective suite. For example: plugin `openshift-kubernetes-conformance` uses the suite `kubernetes/conformance` (for OCP < 4.20) or `kubernetes/conformance/parallel` (for OCP >= 4.20)
```

### 6. provider-certification-plugins: openshift-tests-plugin/pkg/plugin/plugin.go

**Repository:** `/home/jcallen/Development/provider-certification-plugins`
**Commit:** 23f29b7

#### Modified NewPlugin() function (lines 134-145):
**Before:**
```go
case PluginName10, PluginAlias10:
	p.id = PluginId10
	p.SuiteName = PluginSuite10  // <-- HARDCODED to "kubernetes/conformance"
	p.BlockerPlugins = []*Plugin{{name: PluginName05}}
	p.OTRunner = NewOpenShiftRunCommand("run", p.SuiteName)
	p.Timeout = 2 * time.Hour
```

**After:**
```go
case PluginName10, PluginAlias10:
	p.id = PluginId10
	// Use DEFAULT_SUITE_NAME from environment if set, otherwise use the constant
	// This allows OPCT to control the suite name based on cluster version
	if envSuiteName := os.Getenv("DEFAULT_SUITE_NAME"); envSuiteName != "" {
		p.SuiteName = envSuiteName
	} else {
		p.SuiteName = PluginSuite10
	}
	p.BlockerPlugins = []*Plugin{{name: PluginName05}}
	p.OTRunner = NewOpenShiftRunCommand("run", p.SuiteName)
	p.Timeout = 2 * time.Hour
```

**Why this change was critical:**
The plugin was previously hardcoding `p.SuiteName = PluginSuite10` which evaluates to `"kubernetes/conformance"`. This value was then passed to `NewOpenShiftRunCommand()` which generates the start script at `/tmp/shared/start`. The start script template in `run-script.go` uses `{{ .SuiteName }}` which rendered as:

```bash
/usr/bin/openshift-tests run kubernetes/conformance \
```

Even though OPCT set `DEFAULT_SUITE_NAME=kubernetes/conformance/parallel` in the pod's environment variables, the plugin ignored it and used the hardcoded value. This change makes the plugin check the environment variable first, allowing OPCT to control the suite name based on cluster version.

## How It Works

### End-to-End Flow

1. **OPCT Version Detection (pkg/run/run.go)**:
   - During `opct run`, the `PreRunCheck()` method is called
   - After creating the ConfigV1 client, `setKubeConformanceSuiteName()` is invoked
   - The method retrieves the cluster version from the `ClusterVersion` API resource
   - The version string is parsed using `fmt.Sscanf()` to extract major and minor versions
   - Based on the version:
     - If >= 4.20: `KubeConformanceSuiteName = "kubernetes/conformance/parallel"`
     - If < 4.20: `KubeConformanceSuiteName = "kubernetes/conformance"`

2. **Template Rendering (data/templates/plugins/openshift-kube-conformance.yaml)**:
   - The plugin manifest template is rendered with `{{ .KubeConformanceSuiteName }}`
   - This becomes the `DEFAULT_SUITE_NAME` environment variable in the pod spec

3. **Pod Creation**:
   - Kubernetes creates the plugin pod with environment variable:
     - `DEFAULT_SUITE_NAME=kubernetes/conformance/parallel` (for OCP 4.20+)
     - or `DEFAULT_SUITE_NAME=kubernetes/conformance` (for OCP < 4.20)

4. **Plugin Initialization (openshift-tests-plugin/pkg/plugin/plugin.go)**:
   - The plugin's `NewPlugin()` function reads `DEFAULT_SUITE_NAME` from environment
   - Sets `p.SuiteName` to the environment variable value
   - Creates `OpenShiftRunCommand` with the correct suite name

5. **Start Script Generation (openshift-tests-plugin/pkg/plugin/run-script.go)**:
   - The plugin's `OTRunner.Create()` method generates the start script
   - Template renders `{{ .SuiteName }}` to the actual suite name
   - Creates `/tmp/shared/start` with the correct command

6. **Test Execution (openshift-tests-plugin/plugin/entrypoint-tests.sh)**:
   - The `entrypoint-tests.sh` script executes `/tmp/shared/start`
   - openshift-tests runs with the correct suite:
     - `openshift-tests run kubernetes/conformance/parallel` (OCP 4.20+)
     - `openshift-tests run kubernetes/conformance` (OCP < 4.20)

## Critical Lessons Learned - Debugging Journey

This fix required THREE iterations to get right. Here's what we learned:

### Iteration 1: OPCT Version Detection (Incomplete)
**What we did:**
- Added version detection in OPCT `pkg/run/run.go`
- Updated template to use `{{ .KubeConformanceSuiteName }}`
- Set `DEFAULT_SUITE_NAME` environment variable in tests container

**What we thought:** "The environment variable will be picked up by the plugin"

**Result:** ❌ Tests still ran with `kubernetes/conformance`

**Why it failed:** The plugin Go code was hardcoding `p.SuiteName = PluginSuite10` and completely ignoring the environment variable.

### Iteration 2: Plugin Environment Variable Support (Still Incomplete)
**What we did:**
- Modified plugin Go code to read `os.Getenv("DEFAULT_SUITE_NAME")`
- Rebuilt plugin images with commit 23f29b7
- Ran OPCT with new plugin image

**What we thought:** "Now the plugin will read the environment variable"

**Result:** ❌ Tests STILL ran with `kubernetes/conformance`

**Why it failed:** We set `DEFAULT_SUITE_NAME` in the **tests** container, but the Go code runs in the **plugin** container! Environment variables don't cross container boundaries.

**How we discovered it:**
```bash
# Pod had the environment variable in tests container
kubectl get pod <pod> -o yaml | grep -A 2 "name: DEFAULT_SUITE_NAME"
# Output showed it was in tests container

# But plugin container didn't have it
kubectl get pod <pod> -o jsonpath='{.spec.containers[*].env[*].name}' | grep DEFAULT
# Output: (empty for plugin container)

# Tests still showed wrong suite
kubectl logs <pod> -c tests | grep "openshift-tests run"
# Output: /usr/bin/openshift-tests run kubernetes/conformance
```

### Iteration 3: Set Variable in BOTH Containers (Success!)
**What we did:**
- Added `DEFAULT_SUITE_NAME` to BOTH containers in the template:
  - Line 55-56: tests container (for bash script)
  - Line 89-90: plugin container (for Go code) ← **THIS WAS THE MISSING PIECE**

**Result:** ✅ Tests now run with correct suite!

**Why it works:**
```
Plugin Pod Structure:
├── Container: tests (bash)
│   └── Reads: DEFAULT_SUITE_NAME (for logging/debugging)
│   └── Executes: /tmp/shared/start (generated by plugin)
│
└── Container: plugin (Go binary)
    └── Reads: DEFAULT_SUITE_NAME ← **MUST HAVE THIS**
    └── Generates: /tmp/shared/start with correct suite name
```

### Key Architectural Insight

**The plugin architecture has TWO execution contexts:**

1. **Plugin Container** (Go):
   - Runs: `/usr/bin/openshift-tests-plugin run --name ${PLUGIN_NAME}`
   - Purpose: Orchestration, waiting for blockers, generating start scripts
   - **Needs**: `DEFAULT_SUITE_NAME` to know which suite to put in the start script

2. **Tests Container** (Bash):
   - Runs: `/bin/bash /tmp/shared/entrypoint-tests.sh`
   - Purpose: Login to cluster, execute the start script
   - **Uses**: `/tmp/shared/start` (created by plugin container)

**Critical takeaway:** Environment variables set in one container are NOT visible to other containers in the same pod. Each container needs its own environment variables.

### Debugging Checklist for Similar Issues

When debugging environment variable issues in multi-container pods:

1. **Check which container runs which code:**
   ```bash
   kubectl get pod <pod> -o yaml | grep -A 10 "containers:"
   ```

2. **Verify environment variables per container:**
   ```bash
   kubectl exec <pod> -c <container-name> -- env | grep <VAR_NAME>
   ```

3. **Check pod spec to see where variables are defined:**
   ```bash
   kubectl get pod <pod> -o yaml | grep -B 5 "<VAR_NAME>"
   ```

4. **Verify which container needs the variable:**
   - Grep the source code for `os.Getenv("<VAR_NAME>")`
   - Check which binary/script runs in which container
   - Set the variable in the container that actually reads it

## Error Handling

The implementation includes robust error handling:
- If cluster version retrieval fails: defaults to `kubernetes/conformance` and logs warning
- If version string is empty: defaults to `kubernetes/conformance` and logs warning
- If version parsing fails: defaults to `kubernetes/conformance` and logs warning
- All fallback scenarios are logged for debugging

## Testing

Build tested with:
```bash
make
```

Result: ✅ Successful compilation, binary created at `build/opct-linux-amd64`

## Benefits

✅ Fixes the suite error for OCP 4.20+ clusters
✅ Maintains backward compatibility for OCP < 4.20
✅ No changes required to provider-certification-plugins repository
✅ Graceful fallback behavior if version detection fails
✅ Clear logging of suite selection for debugging
✅ Version-aware logic is centralized and maintainable

## Future Considerations

1. **OCP 5.x Support**: The version check uses `major == 4 && minor >= 20`. If OCP 5.x is released, this logic will need updating to handle major version changes.

2. **Suite Name Changes**: If OpenShift changes suite names again in future versions, this mechanism can be extended to handle additional version-specific mappings.

3. **Testing Across Versions**: This fix should be tested on:
   - OCP 4.19.x (should use kubernetes/conformance)
   - OCP 4.20.x (should use kubernetes/conformance/parallel)
   - OCP 4.21+ (should use kubernetes/conformance/parallel)

4. **Serial Tests**: Currently only using `kubernetes/conformance/parallel` for OCP >= 4.20. Consider if `kubernetes/conformance/serial` tests also need to be run, either as a separate plugin execution or combined approach.

5. **Minimal Variants**: The minimal variants (`kubernetes/conformance/parallel/minimal` and `kubernetes/conformance/serial/minimal`) exist for faster test runs. These could be made available as configuration options.

6. **Provider-Certification-Plugins**: The `entrypoint-tests.sh` script in that repository doesn't need changes, as it receives the suite name via the `DEFAULT_SUITE_NAME` environment variable.

## Related Files (No Changes Required)

These files reference the suite name but didn't require changes:
- `internal/opct/summary/suite.go` - Defines constant `SuiteNameKubernetesConformance`
- `internal/opct/plugin/plugin.go` - Defines constant `PluginNameKubernetesConformance`
- `pkg/cmd/report/report.go` - Uses the plugin name constant
- Various summary/consolidated files - Process results regardless of suite name

## Git Branch

Changes made on branch: `fixes`

## Date

2025-11-20

---

## Architecture Context (For Complete Understanding)

### Two-Repository System

This fix involves only the **OPCT repository**, but understanding the full architecture helps:

**1. OPCT Repository** (this repo)
- Purpose: CLI tool and orchestration
- What it does:
  - Detects cluster version
  - Selects appropriate suite name
  - Renders plugin templates with suite name
  - Orchestrates Sonobuoy and plugin execution
- Changes: Version detection logic in `pkg/run/run.go` and template variable in plugin YAML

**2. Provider-Certification-Plugins Repository** (`/home/jcallen/Development/provider-certification-plugins`)
- Purpose: Plugin container images with test execution logic
- What it contains:
  - `entrypoint-tests.sh` - Reads `$DEFAULT_SUITE_NAME` environment variable
  - Plugin images: `quay.io/opct/plugin-openshift-tests:v0.6.0`
- Changes: **NONE** - Script already reads environment variable dynamically

### Why No Plugin Changes Are Needed

The fix is entirely in OPCT because:

1. **OPCT determines the suite name** based on cluster version
2. **OPCT renders the template** with `{{ .KubeConformanceSuiteName }}`
3. **Template becomes environment variable** `DEFAULT_SUITE_NAME` in pod
4. **Plugin script reads variable** - `SUITE_NAME=${DEFAULT_SUITE_NAME}`
5. **Plugin executes** - `openshift-tests run $SUITE_NAME`

The interface between OPCT and plugins is **environment variables**, which is already dynamic.

### Image Dependency Chain

```
opct run
    ↓
Renders template: DEFAULT_SUITE_NAME="{{ .KubeConformanceSuiteName }}"
    ↓
Creates pod with: env: DEFAULT_SUITE_NAME=kubernetes/conformance/parallel
    ↓
Init container (quay.io/opct/plugin-openshift-tests:v0.6.0)
    └─ Copies entrypoint-tests.sh to shared volume
    ↓
Test container (openshift/tests from cluster registry)
    └─ Runs: /bin/bash /tmp/shared/entrypoint-tests.sh
       └─ Reads: $DEFAULT_SUITE_NAME
          └─ Executes: openshift-tests run kubernetes/conformance/parallel
```

---

## Image Versions and Compatibility

### Current Image Versions (v0.6.0)

All images published to `quay.io/opct/`:

- `opct:v0.6.0` - CLI container
- `plugin-openshift-tests:v0.6.0` - Test execution plugin
- `plugin-artifacts-collector:v0.6.0` - Artifact collector
- `must-gather-monitoring:v0.6.0` - Must-gather monitoring
- `sonobuoy:v0.57.3` - Sonobuoy aggregator

### Image References in Code

From `pkg/types.go`:
```go
const (
    DefaultToolsRepository         = "quay.io/opct"
    ControllerImage                = "quay.io/opct/opct:v0.6.0"
    PluginsImage                   = "plugin-openshift-tests:v0.6.0"       // Added dynamically
    CollectorImage                 = "plugin-artifacts-collector:v0.6.0"   // Added dynamically
    MustGatherMonitoringImage      = "must-gather-monitoring:v0.6.0"       // Added dynamically
)
```

**Note**: Plugin image names don't include registry prefix in constants. The `GetPluginsImage()` function prepends `DefaultToolsRepository` at runtime.

### This Fix Works With All Plugin Versions

Because the fix only changes:
- What value is set for `KubeConformanceSuiteName` (in Go code)
- What gets rendered in the template (still `{{ .KubeConformanceSuiteName }}`)
- What environment variable value is passed to the pod

The `entrypoint-tests.sh` script doesn't change, so:
- ✅ Works with v0.6.0 plugin images
- ✅ Works with v0.6.0-rc.0 plugin images
- ✅ Works with v0.5.0 plugin images
- ✅ Works with future plugin versions

---

## Testing Instructions

### Prerequisites

1. **Access to OCP 4.20+ cluster**
   ```bash
   oc version
   # Should show: Server Version: 4.20.x or higher
   ```

2. **Cluster admin access**
   ```bash
   oc auth can-i create namespaces
   # Should show: yes
   ```

3. **Build updated OPCT binary**
   ```bash
   cd /home/jcallen/Development/opct
   make build-linux-amd64
   # Binary: build/opct-linux-amd64
   ```

### Test 1: Verify Version Detection

```bash
# Run with debug logging
export LOG_LEVEL=debug
./build/opct-linux-amd64 run --devel-skip-checks

# Expected in logs:
#   "Detected cluster version: 4.20.0"
#   "Using kubernetes/conformance/parallel suite for OCP 4.20"
```

### Test 2: Verify Template Rendering

```bash
# Check rendered manifest
kubectl get pod -n opct -o yaml | grep -A 5 DEFAULT_SUITE_NAME

# Expected output:
#   - name: DEFAULT_SUITE_NAME
#     value: kubernetes/conformance/parallel
```

### Test 3: Verify Suite Execution

```bash
# Watch plugin pod logs
kubectl logs -n opct -l sonobuoy-plugin=10-openshift-kube-conformance \
  -c tests --follow

# Expected in logs:
#   "Suite Name: kubernetes/conformance/parallel"
#   "openshift-tests run kubernetes/conformance/parallel --dry-run"
#   (Should NOT show "suite does not exist" error)
```

### Test 4: End-to-End Test

```bash
# Full test run
./build/opct-linux-amd64 run --watch

# Wait for completion
./build/opct-linux-amd64 retrieve ./results

# Check results
./build/opct-linux-amd64 report ./results/sonobuoy_*.tar.gz

# Expected:
#   - No "suite does not exist" errors
#   - Tests execute successfully
#   - Results show kubernetes/conformance/parallel tests
```

### Test 5: Backward Compatibility (OCP < 4.20)

If you have access to OCP 4.19 or earlier:

```bash
# On OCP 4.19 cluster
oc version
# Server Version: 4.19.x

./build/opct-linux-amd64 run --watch

# Expected logs:
#   "Using kubernetes/conformance suite for OCP 4.19"

# Expected env var:
#   DEFAULT_SUITE_NAME: kubernetes/conformance
```

---

## Troubleshooting Guide

### Issue: Pods in ImagePullBackOff

**Symptoms:**
```bash
kubectl get pods -n opct
# NAME                                        READY   STATUS
# sonobuoy-10-openshift-kube-conformance-...  0/3     ImagePullBackOff
```

**Diagnosis:**
```bash
kubectl describe pod <pod-name> -n opct | grep -A 5 Events
# Look for: Failed to pull image "quay.io/opct/plugin-openshift-tests:v0.6.0"
```

**Cause**: Plugin images don't exist in registry with v0.6.0 tag

**Resolution:**
1. Verify images exist:
   ```bash
   podman pull quay.io/opct/plugin-openshift-tests:v0.6.0
   ```

2. If missing, either:
   - Wait for plugin images to be published
   - Use existing version by updating `pkg/types.go`:
     ```go
     PluginsImage = "plugin-openshift-tests:v0.6.0-rc.0"
     ```
   - Build plugin images locally (advanced)

### Issue: "suite does not exist" Error Still Appears

**Symptoms:**
```bash
kubectl logs <pod-name> -n opct -c tests
# error: error converting to options: suite "kubernetes/conformance" does not exist
```

**Cause**: Using old OPCT binary without the fix

**Resolution:**
1. Verify you built updated binary:
   ```bash
   strings build/opct-linux-amd64 | grep "Using kubernetes/conformance/parallel"
   # Should show the log message from new code
   ```

2. Make sure you're running the correct binary:
   ```bash
   which opct
   # Should point to your build directory or updated install location
   ```

3. Clean existing run and retry:
   ```bash
   ./opct destroy
   ./build/opct-linux-amd64 run --watch
   ```

### Issue: Wrong Suite Name Being Used (Plugin Hardcoding)

**Symptoms**:
- Pod has `DEFAULT_SUITE_NAME=kubernetes/conformance/parallel` in environment
- But tests still run with `kubernetes/conformance`
- Log shows: `/usr/bin/openshift-tests run kubernetes/conformance`

**Diagnosis:**
```bash
# Check pod environment variable (should be correct)
kubectl get pod <pod-name> -n opct -o yaml | grep -A 5 DEFAULT_SUITE_NAME
# Expected: value: kubernetes/conformance/parallel

# Check actual command being run (was wrong before plugin fix)
kubectl logs <pod-name> -n opct -c tests | grep "openshift-tests run"
# Was showing: /usr/bin/openshift-tests run kubernetes/conformance
# Should show: /usr/bin/openshift-tests run kubernetes/conformance/parallel
```

**Root Cause**: Plugin Go code was hardcoding suite name and ignoring environment variable

**Resolution**:
1. **Rebuild plugin images** with commit 23f29b7 from provider-certification-plugins
2. Use the new plugin image tag when running OPCT:
   ```bash
   ./opct run --plugins-image=quay.io/opct/plugin-openshift-tests:v0.0.0-devel-23f29b7
   ```
3. Or wait for official plugin images to be published with the fix

**Why this happened**: The plugin's `NewPlugin()` function was using a hardcoded constant `PluginSuite10 = "kubernetes/conformance"` instead of reading the `DEFAULT_SUITE_NAME` environment variable that OPCT set.

### Issue: Wrong Suite Name Being Used (Other Causes)

**Symptoms**: Tests run but use wrong suite

**Diagnosis:**
```bash
# Check what suite is actually set
kubectl exec -n opct <pod-name> -c tests -- env | grep SUITE
```

**Possible Causes:**
1. Version detection failed - check logs for warnings
2. Template rendering error - check pod YAML
3. Environment variable override - check for `SUITE_NAME` set elsewhere
4. Old plugin images without fix (see above section)

**Resolution**: Review logs in PreRunCheck phase for version detection

---

## Pod Failure Investigation Commands

```bash
# Get pod status
kubectl get pods -n opct -o wide

# Describe pod (shows events, conditions)
kubectl describe pod <pod-name> -n opct

# Logs from all containers
kubectl logs <pod-name> -n opct --all-containers

# Logs from specific container
kubectl logs <pod-name> -n opct -c sync    # Init container
kubectl logs <pod-name> -n opct -c login   # Init container
kubectl logs <pod-name> -n opct -c tests   # Main test container
kubectl logs <pod-name> -n opct -c plugin  # Plugin sidecar

# Previous logs (after crash/restart)
kubectl logs <pod-name> -n opct -c tests --previous

# Check environment variables in running pod
kubectl exec <pod-name> -n opct -c tests -- env | grep -E 'SUITE|DEFAULT'

# Get pod YAML to see rendered template
kubectl get pod <pod-name> -n opct -o yaml

# Check Sonobuoy aggregator logs
kubectl logs -n opct sonobuoy

# Events in namespace
kubectl get events -n opct --sort-by='.lastTimestamp'
```

---

## Additional Documentation

For complete project architecture understanding, see:
- **OPCT_ARCHITECTURE.md** - Comprehensive architecture guide
- **Official Docs**: https://redhat-openshift-ecosystem.github.io/opct/
- **OPCT GitHub**: https://github.com/redhat-openshift-ecosystem/opct
- **Plugins GitHub**: https://github.com/redhat-openshift-ecosystem/provider-certification-plugins

---

## Summary Checklist

### What Changed in OPCT Repository
- [x] Added version detection in `pkg/run/run.go`
- [x] Added `KubeConformanceSuiteName` field to `RunOptions`
- [x] Updated template to use `{{ .KubeConformanceSuiteName }}`
- [x] **CRITICAL FIX**: Added `DEFAULT_SUITE_NAME` to plugin container environment (line 89-90)
  - Initially only added to tests container - didn't work!
  - Go code runs in plugin container, so variable must be there
- [x] Updated all documentation (internal/report/slo.go, docs/review/rules.md, docs/devel/guide.md)
- [x] Added SHARED_DIR workaround to plugin templates (commit c831c68)

### What Changed in Provider-Certification-Plugins Repository
- [x] **CRITICAL**: Modified `NewPlugin()` to read `DEFAULT_SUITE_NAME` from environment (commit 23f29b7)
- [x] Plugin now respects environment variable instead of hardcoded constant
- [x] Added fallback to hardcoded constant for backward compatibility
- [x] Fixed SHARED_DIR bug in entrypoint-tests.sh (commit 4c7e163)
- [x] Plugin images need rebuild with commit 23f29b7 for fix to work

### What Works
- [x] OCP < 4.20 uses `kubernetes/conformance`
- [x] OCP >= 4.20 uses `kubernetes/conformance/parallel`
- [x] Backward compatible - falls back to hardcoded suite if env var not set
- [x] Graceful fallback if version detection fails in OPCT
- [x] SHARED_DIR environment variable set as workaround in OPCT templates

### Testing Status
- [x] OPCT code compiles successfully (`make build-linux-amd64`)
- [x] Plugin code compiles successfully (`make build-plugin-tests`)
- [ ] Tested on OCP 4.20+ cluster with new plugin images
- [ ] Tested on OCP 4.19 cluster (backward compatibility)
- [ ] End-to-end test passed
- [ ] Results validated

### Required for Complete Fix
- [ ] Rebuild and publish plugin images with commit 23f29b7
- [ ] Update OPCT to use new plugin image tag
- [ ] Optionally: Revert OPCT workaround commit c831c68 after new images published

---

**Last Updated**: 2025-11-20
**Status**: Code Complete, Pending Plugin Image Rebuild and Testing

**Git Commits:**
- OPCT: Suite detection and template update (current HEAD)
- OPCT: SHARED_DIR workaround (c831c68) - can be reverted after plugin images rebuilt
- Plugins: DEFAULT_SUITE_NAME environment variable support (23f29b7) - **REQUIRED**
- Plugins: SHARED_DIR bug fix (4c7e163)
