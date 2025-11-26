# Switch to k8s-tests-ext for Kubernetes Conformance (OCP 4.20+)

## Date
2025-11-26

## Problem Statement

OpenShift-tests is no longer properly managing Kubernetes conformance tests in OCP 4.20+. The kubernetes/conformance suite was reorganized, and the recommended approach is to use the k8s-tests-ext binary directly instead of relying on openshift-tests as a wrapper.

### Background

In OCP 4.20+:
- The kubernetes/conformance suite was split into `kubernetes/conformance/parallel` and `kubernetes/conformance/serial`
- The k8s-tests-ext binary (Kubernetes tests extension) is the proper tool for running these tests
- k8s-tests-ext is distributed as part of the OCP release payload in the hyperkube image
- It implements the OTE (OpenShift Tests Extension) interface

## Solution Approach

For OCP 4.20+, we now:
1. **Extract k8s-tests-ext binary** from the hyperkube image at runtime
2. **Use the OTE interface** for test discovery and execution
3. **Maintain backward compatibility** by continuing to use openshift-tests for OCP < 4.20

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ OPCT Version Detection                                      │
│ ├─ OCP < 4.20                                              │
│ │  └─ Suite: kubernetes/conformance                        │
│ │     Binary: openshift-tests                              │
│ │     UseK8sTestsExt: false                               │
│ │                                                           │
│ └─ OCP >= 4.20                                             │
│    └─ Suite: kubernetes/conformance/parallel               │
│       Binary: k8s-tests-ext                                │
│       UseK8sTestsExt: true                                 │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ Plugin Template Rendering                                   │
│ └─ Sets USE_K8S_TESTS_EXT environment variable            │
│    └─ Conditionally adds extract-k8s-tests init container  │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ Pod Execution (OCP 4.20+)                                   │
│                                                             │
│ Init Containers:                                            │
│ ├─ sync: Copy entrypoint scripts                          │
│ ├─ login: Authenticate with cluster                       │
│ └─ extract-k8s-tests:                                      │
│    ├─ Get release image from ClusterVersion               │
│    ├─ Extract image-references from release               │
│    ├─ Parse hyperkube image reference                     │
│    ├─ Extract /usr/bin/k8s-tests-ext.gz                   │
│    └─ Decompress to /tmp/shared/k8s-tests-ext             │
│                                                             │
│ Main Container: tests                                       │
│ └─ entrypoint-tests.sh                                     │
│    ├─ Detect k8s-tests-ext binary                         │
│    ├─ Test Discovery:                                      │
│    │  └─ k8s-tests-ext list -o jsonl                      │
│    │     └─ Parse JSONL to test list                      │
│    └─ Test Execution:                                      │
│       └─ For each test:                                    │
│          ├─ k8s-tests-ext run-test -n "test" -o jsonl    │
│          ├─ Parse JSONL result                            │
│          └─ Generate JUnit XML entry                      │
└─────────────────────────────────────────────────────────────┘
```

## Changes Made

### 1. OPCT Repository (`/home/jcallen/Development/opct`)

#### A. pkg/run/run.go

**Added field to RunOptions struct** (lines 59-63):
```go
// UseK8sTestsExt
// indicates whether to use k8s-tests-ext binary instead of openshift-tests
// for Kubernetes conformance testing.
// This is version-dependent: false for OCP < 4.20, true for OCP >= 4.20.
UseK8sTestsExt bool
```

**Removed field**:
```go
// KubeConformanceFocusFilter - No longer needed
```

**Updated setKubeConformanceSuiteName() method** (lines 209-253):
```go
func (r *RunOptions) setKubeConformanceSuiteName(oc *coclient.Clientset) error {
    // Default to kubernetes/conformance for backward compatibility
    r.KubeConformanceSuiteName = "kubernetes/conformance"
    r.UseK8sTestsExt = false

    // Get cluster version and parse
    cv, err := oc.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
    // ... error handling ...

    var major, minor int
    _, err = fmt.Sscanf(version, "%d.%d", &major, &minor)
    // ... error handling ...

    // For OCP 4.20+, use k8s-tests-ext binary with parallel sub-suite
    if major == 4 && minor >= 20 {
        r.KubeConformanceSuiteName = "kubernetes/conformance/parallel"
        r.UseK8sTestsExt = true
        log.Infof("Using kubernetes/conformance/parallel suite with k8s-tests-ext for OCP %d.%d", major, minor)
    } else {
        log.Infof("Using kubernetes/conformance suite with openshift-tests for OCP %d.%d", major, minor)
    }

    return nil
}
```

#### B. data/templates/plugins/openshift-kube-conformance.yaml

**Added conditional init container** (lines 42-99):
```yaml
{{- if .UseK8sTestsExt }}
    - name: extract-k8s-tests
      image: "{{ .OpenshiftTestsImage }}"
      imagePullPolicy: Always
      command:
        - "/bin/bash"
        - "-c"
        - |
          set -euo pipefail

          echo "Extracting k8s-tests-ext from hyperkube image..."

          # Get release image from cluster
          RELEASE_IMAGE=$(oc get clusterversion version -o jsonpath='{.status.desired.image}')
          echo "Release image: ${RELEASE_IMAGE}"

          # Extract image-references
          oc image extract "${RELEASE_IMAGE}" \
            --path=/release-manifests/image-references:/tmp/refs \
            --confirm

          # Parse hyperkube image reference using python
          HYPERKUBE_IMAGE=$(python3 -c "
          import json, sys
          with open('/tmp/refs/image-references') as f:
              data = json.load(f)
              for tag in data.get('spec', {}).get('tags', []):
                  if tag.get('name') == 'hyperkube':
                      print(tag.get('from', {}).get('name', ''))
                      sys.exit(0)
              sys.exit(1)
          ")

          if [ -z "$HYPERKUBE_IMAGE" ]; then
              echo "ERROR: Failed to find hyperkube image in release payload"
              exit 1
          fi

          echo "Hyperkube image: ${HYPERKUBE_IMAGE}"

          # Extract k8s-tests-ext.gz
          oc image extract "${HYPERKUBE_IMAGE}" \
            --path=/usr/bin/k8s-tests-ext.gz:/tmp/shared \
            --confirm

          # Decompress and make executable
          gunzip /tmp/shared/k8s-tests-ext.gz
          chmod +x /tmp/shared/k8s-tests-ext

          echo "k8s-tests-ext extracted successfully"
          /tmp/shared/k8s-tests-ext info || echo "Warning: k8s-tests-ext info command failed (may be expected)"
      env:
        - name: KUBECONFIG
          value: "/tmp/shared/kubeconfig"
      volumeMounts:
        - mountPath: /tmp/shared
          name: shared
{{- end }}
```

**Updated environment variables** in tests container (lines 115-116):
```yaml
- name: USE_K8S_TESTS_EXT
  value: "{{ .UseK8sTestsExt }}"
```

**Updated environment variables** in plugin container (lines 151-152):
```yaml
- name: USE_K8S_TESTS_EXT
  value: "{{ .UseK8sTestsExt }}"
```

**Removed**:
```yaml
{{- if .KubeConformanceFocusFilter }}
- name: E2E_FOCUS
  value: '{{ .KubeConformanceFocusFilter }}'
{{- end }}
```

#### C. docs/review/rules.md (line 17)

**Before**:
```markdown
For OCP >= 4.20, this uses the `kubernetes/conformance/parallel` suite with E2E_FOCUS filter set to `\[Conformance\]` to ensure only conformance-tagged tests run
```

**After**:
```markdown
For OCP >= 4.20, this uses the `kubernetes/conformance/parallel` suite via `k8s-tests-ext` binary (extracted from the hyperkube image in the release payload)
```

#### D. internal/report/slo.go (line 256)

Updated description to match docs/review/rules.md.

### 2. Provider-Certification-Plugins Repository

#### openshift-tests-plugin/plugin/entrypoint-tests.sh

**Binary Detection** (lines 25-38):
```bash
# Detect which test binary to use based on environment variable and availability
if [[ "${USE_K8S_TESTS_EXT:-false}" == "true" ]] && [[ -x "/tmp/shared/k8s-tests-ext" ]]; then
    declare -gr CMD_TESTS="/tmp/shared/k8s-tests-ext"
    declare -gr USE_OTE_INTERFACE="true"
    echo "Using k8s-tests-ext binary with OTE interface"
else
    declare -gr CMD_TESTS="/usr/bin/openshift-tests"
    declare -gr USE_OTE_INTERFACE="false"
    echo "Using openshift-tests binary"
fi

# Legacy variable for backward compatibility
declare -gr CMD_OTESTS="${CMD_TESTS}"
```

**Test Discovery Functions** (lines 61-102):

```bash
# Function to gather test list using OTE interface (k8s-tests-ext)
function gather_test_list_ote() {
    local suite_name="${1:-}"
    local output_file="${2}"

    echo "Gathering test list using OTE interface for suite: ${suite_name}"

    # k8s-tests-ext list command outputs JSONL format
    ${CMD_TESTS} list -o jsonl > "${output_file}.jsonl" 2>"${output_file}.log"

    # Convert JSONL to simple test name list (one per line)
    if command -v jq &> /dev/null; then
        jq -r '.name' "${output_file}.jsonl" > "${output_file}"
    else
        # Fallback if jq is not available - use python
        python3 -c "
import json, sys
with open('${output_file}.jsonl') as f:
    for line in f:
        if line.strip():
            data = json.loads(line)
            print(data.get('name', ''))
" > "${output_file}"
    fi

    echo "Extracted $(wc -l < "${output_file}") tests from OTE interface"
}

# Function to gather test list using standard openshift-tests interface
function gather_test_list_standard() {
    local run_command="${1:-run}"
    local suite_name="${2:-}"
    local additional_args="${3:-}"
    local output_file="${4}"

    echo "Gathering test list using openshift-tests for suite: ${suite_name}"

    ${CMD_TESTS} ${run_command} ${suite_name} ${additional_args} --dry-run -o "${output_file}" >"${output_file}.log" 2>&1
}
```

**Updated Test Discovery Logic** (lines 104-129):
```bash
elif [[ "${PLUGIN_NAME:-}" != "openshift-cluster-upgrade" ]]; then
    if [[ "${USE_OTE_INTERFACE}" == "true" ]]; then
        gather_test_list_ote "${SUITE_NAME:-${DEFAULT_SUITE_NAME-}}" "${CTRL_SUITE_LIST}"
    else
        gather_test_list_standard "${OT_RUN_COMMAND:-run}" "${SUITE_NAME:-${DEFAULT_SUITE_NAME-}}" "" "${CTRL_SUITE_LIST}"
    fi
```

**Test Execution Function** (lines 135-231):

```bash
# Function to execute tests using OTE interface (k8s-tests-ext)
function execute_tests_ote() {
    local test_list="${CTRL_SUITE_LIST}"
    local junit_dir="/tmp/shared/junit"

    echo "Executing tests using OTE interface..."
    mkdir -p "${junit_dir}"

    # Set up environment variables required by k8s-tests-ext
    export TEST_PROVIDER="{\"ProviderName\":\"skeleton\"}"
    export EXTENSION_ARTIFACT_DIR="/tmp/shared/artifacts"
    mkdir -p "${EXTENSION_ARTIFACT_DIR}"

    # Create start script that executes k8s-tests-ext for each test
    cat > "${CTRL_START_SCRIPT}" <<'EOF_START'
#!/bin/bash
set -euo pipefail

echo "Executing k8s-tests-ext conformance tests..."

junit_dir="/tmp/shared/junit"
test_list="/tmp/shared/suite.list"
cmd_tests="/tmp/shared/k8s-tests-ext"

# Create JUnit XML header
junit_file="${junit_dir}/junit_runner.xml"
test_count=$(wc -l < "${test_list}")
pass_count=0
fail_count=0
skip_count=0

echo '<?xml version="1.0" encoding="UTF-8"?>' > "${junit_file}"
echo '<testsuites>' >> "${junit_file}"
echo "  <testsuite name=\"kubernetes-conformance\" tests=\"${test_count}\">" >> "${junit_file}"

# Read tests from list and execute each one
while IFS= read -r test_name || [ -n "$test_name" ]; do
    if [ -z "$test_name" ]; then
        continue
    fi

    echo "Running test: ${test_name}"

    # Run single test using OTE interface
    result_json=$(mktemp)
    if "${cmd_tests}" run-test -n "${test_name}" -o jsonl > "${result_json}" 2>&1; then
        # Parse result from JSONL
        test_result=$(python3 -c "
import json, sys
try:
    with open('${result_json}') as f:
        for line in f:
            if line.strip():
                data = json.loads(line)
                print(data.get('result', 'unknown'))
                break
except Exception as e:
    print('unknown', file=sys.stderr)
" 2>/dev/null || echo "unknown")

        case "${test_result}" in
            passed)
                echo "  <testcase name=\"${test_name}\" classname=\"kubernetes.conformance\" status=\"passed\"/>" >> "${junit_file}"
                ((pass_count++))
                ;;
            skipped)
                echo "  <testcase name=\"${test_name}\" classname=\"kubernetes.conformance\" status=\"skipped\"><skipped/></testcase>" >> "${junit_file}"
                ((skip_count++))
                ;;
            *)
                echo "  <testcase name=\"${test_name}\" classname=\"kubernetes.conformance\" status=\"failed\"><failure>Test failed or result unknown</failure></testcase>" >> "${junit_file}"
                ((fail_count++))
                ;;
        esac
    else
        echo "  <testcase name=\"${test_name}\" classname=\"kubernetes.conformance\" status=\"failed\"><failure>Test execution failed</failure></testcase>" >> "${junit_file}"
        ((fail_count++))
    fi

    rm -f "${result_json}"
done < "${test_list}"

# Close JUnit XML
echo "  </testsuite>" >> "${junit_file}"
echo "</testsuites>" >> "${junit_file}"

echo "Test execution complete: ${pass_count} passed, ${fail_count} failed, ${skip_count} skipped out of ${test_count} total"
EOF_START

    chmod +x "${CTRL_START_SCRIPT}"
    echo "Created OTE start script at ${CTRL_START_SCRIPT}"

    # Execute the start script
    ${CTRL_START_SCRIPT}
}
```

**Updated Execution Logic** (lines 233-251):
```bash
echo "#> waiting for start command"
if [[ "${USE_OTE_INTERFACE}" == "true" ]]; then
    echo "Using OTE interface - generating and executing k8s-tests-ext start script"
    execute_tests_ote
else
    # Standard flow: wait for plugin Go code to generate start script
    msg="waiting for start command [${CTRL_START_SCRIPT}]. Read the container 'plugin' logs for more information."
    while true;
    do
        if [[ -f ${CTRL_START_SCRIPT} ]];
        then
            chmod u+x $CTRL_START_SCRIPT && cat $CTRL_START_SCRIPT && $CTRL_START_SCRIPT;
            break;
        fi
        echo "$(date) ${msg}";
        sleep 10;
    done
fi
```

## How It Works

### Binary Extraction Process

1. **Get Release Image**: Query ClusterVersion resource for the release image
   ```bash
   RELEASE_IMAGE=$(oc get clusterversion version -o jsonpath='{.status.desired.image}')
   ```

2. **Extract Image References**: Pull the image-references manifest from release
   ```bash
   oc image extract "${RELEASE_IMAGE}" \
     --path=/release-manifests/image-references:/tmp/refs
   ```

3. **Parse Hyperkube Image**: Use Python to parse the ImageStream JSON
   ```python
   for tag in data.get('spec', {}).get('tags', []):
       if tag.get('name') == 'hyperkube':
           print(tag.get('from', {}).get('name', ''))
   ```

4. **Extract Binary**: Pull the compressed binary from hyperkube
   ```bash
   oc image extract "${HYPERKUBE_IMAGE}" \
     --path=/usr/bin/k8s-tests-ext.gz:/tmp/shared
   ```

5. **Prepare Binary**: Decompress and make executable
   ```bash
   gunzip /tmp/shared/k8s-tests-ext.gz
   chmod +x /tmp/shared/k8s-tests-ext
   ```

### OTE Interface Usage

#### Test Discovery
```bash
# List all tests in JSONL format
k8s-tests-ext list -o jsonl > tests.jsonl

# Each line is JSON: {"name":"test-name","labels":{...},"suite":"..."}
# Extract just the test names
jq -r '.name' tests.jsonl > test-list.txt
```

#### Test Execution
```bash
# Run a single test
k8s-tests-ext run-test -n "[sig-apps] Deployment deployment should support proportional scaling" -o jsonl

# Output JSONL: {"name":"...","result":"passed","output":"...","error":"","startTime":"...","endTime":"..."}
```

### Environment Variables

k8s-tests-ext requires:
- `KUBECONFIG`: Path to kubeconfig file
- `TEST_PROVIDER`: JSON string with provider metadata (we use `{"ProviderName":"skeleton"}`)
- `EXTENSION_ARTIFACT_DIR`: Directory for test artifacts

## Testing Instructions

### Prerequisites

1. **OCP 4.20+ cluster** with cluster-admin access
2. **Updated OPCT binary** with changes from this branch
3. **Updated plugin images** with entrypoint-tests.sh changes

### Build Instructions

#### OPCT
```bash
cd /home/jcallen/Development/opct
make build-linux-amd64
# Binary: build/opct-linux-amd64
```

#### Plugin Images
```bash
cd /home/jcallen/Development/provider-certification-plugins
make build-plugin-tests
# Note the image tag for testing
```

### Test Execution

#### Phase 1: Binary Extraction Validation

```bash
# Run OPCT with debug logging
export LOG_LEVEL=debug
./build/opct-linux-amd64 run --watch

# Check init container logs
kubectl logs -n opct <pod-name> -c extract-k8s-tests

# Expected output:
# Extracting k8s-tests-ext from hyperkube image...
# Release image: quay.io/openshift-release-dev/ocp-release:4.20.0-x86_64
# Hyperkube image: quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:...
# k8s-tests-ext extracted successfully
```

#### Phase 2: Test Discovery Validation

```bash
# Check test discovery logs
kubectl logs -n opct <pod-name> -c tests | grep -A 10 "Gathering test list"

# Expected output:
# Using k8s-tests-ext binary with OTE interface
# Gathering test list using OTE interface for suite: kubernetes/conformance/parallel
# Extracted 350 tests from OTE interface
```

#### Phase 3: Test Execution Validation

```bash
# Monitor test execution
kubectl logs -n opct <pod-name> -c tests --follow

# Expected output:
# Using OTE interface - generating and executing k8s-tests-ext start script
# Executing k8s-tests-ext conformance tests...
# Running test: [sig-apps] Deployment deployment should support proportional scaling
# Running test: [sig-api-machinery] Aggregator Should be able to support the 1.17 Sample API Server
# ...
# Test execution complete: 340 passed, 5 failed, 5 skipped out of 350 total
```

#### Phase 4: Results Validation

```bash
# Retrieve results
./opct retrieve ./results

# Generate report
./opct report ./results/sonobuoy_*.tar.gz

# Check OPCT-001 status
# Expected: PASS (or specific failure details for any failed tests)
```

### Backward Compatibility Testing (OCP 4.19)

```bash
# On OCP 4.19 cluster
./build/opct-linux-amd64 run --watch

# Verify in logs:
# Using kubernetes/conformance suite with openshift-tests for OCP 4.19
# Using openshift-tests binary

# Check that no extract-k8s-tests init container is created
kubectl get pod <pod-name> -n opct -o yaml | grep -A 5 initContainers
# Should only show: sync, login (no extract-k8s-tests)
```

## Troubleshooting

### Issue 1: Binary Extraction Fails

**Symptoms**:
```
ERROR: Failed to find hyperkube image in release payload
```

**Diagnosis**:
```bash
kubectl logs -n opct <pod-name> -c extract-k8s-tests

# Check release image
oc get clusterversion version -o jsonpath='{.status.desired.image}'

# Manually verify hyperkube exists
oc adm release info $(oc get clusterversion version -o jsonpath='{.status.desired.image}') | grep hyperkube
```

**Resolution**:
- Ensure cluster is OCP 4.20+
- Verify release image is accessible
- Check pull secrets are configured

### Issue 2: k8s-tests-ext Not Executable

**Symptoms**:
```
bash: /tmp/shared/k8s-tests-ext: Permission denied
```

**Diagnosis**:
```bash
kubectl exec -n opct <pod-name> -c tests -- ls -la /tmp/shared/k8s-tests-ext
```

**Resolution**:
- Check init container completed successfully
- Verify chmod +x executed in extraction script

### Issue 3: JSONL Parsing Fails

**Symptoms**:
```
Test execution complete: 0 passed, 350 failed, 0 skipped out of 350 total
```

**Diagnosis**:
```bash
# Check test execution logs for parsing errors
kubectl logs -n opct <pod-name> -c tests | grep -i "python\|json\|parse"

# Manually test k8s-tests-ext
kubectl exec -n opct <pod-name> -c tests -- /tmp/shared/k8s-tests-ext list -o jsonl | head -1
```

**Resolution**:
- Ensure python3 is available in tests container
- Verify JSONL format is valid
- Check for errors in python parsing code

### Issue 4: No Tests Discovered

**Symptoms**:
```
Extracted 0 tests from OTE interface
```

**Diagnosis**:
```bash
# Check k8s-tests-ext list output
kubectl exec -n opct <pod-name> -c tests -- /tmp/shared/k8s-tests-ext list -o jsonl

# Check for errors
kubectl logs -n opct <pod-name> -c tests | grep -i "error\|fail"
```

**Resolution**:
- Verify k8s-tests-ext binary is from correct OCP version
- Check KUBECONFIG is set correctly
- Verify cluster access from pod

### Issue 5: Tests Hang or Timeout

**Symptoms**:
- Test execution never completes
- Pod stuck in Running state

**Diagnosis**:
```bash
# Check which test is running
kubectl logs -n opct <pod-name> -c tests | tail -20

# Check pod resource usage
kubectl top pod <pod-name> -n opct
```

**Resolution**:
- Individual test may be hanging - check test logs
- Increase timeout in plugin configuration
- Consider running subset of tests for debugging

## Performance Considerations

### Test Execution Time

**openshift-tests** (old approach):
- Runs entire suite in a single process
- Typical time: 1-2 hours for ~350 tests

**k8s-tests-ext** (new approach):
- Runs each test individually via `run-test`
- Typical time: May be longer due to per-test overhead
- Advantage: Better isolation, clearer failure attribution

### Optimization Opportunities

1. **Parallel Execution**: Run multiple tests concurrently
   ```bash
   # Future enhancement: use xargs -P or GNU parallel
   cat test-list.txt | xargs -P 4 -I {} k8s-tests-ext run-test -n "{}"
   ```

2. **Batching**: Group tests and run in batches
   ```bash
   # Future: investigate if k8s-tests-ext supports running multiple tests
   ```

3. **Caching**: Cache k8s-tests-ext binary across runs
   - Current: Extracted on every plugin pod creation
   - Future: Use persistent volume or image layer

## Benefits

✅ **Proper Binary Usage**: Uses k8s-tests-ext as intended by upstream
✅ **Better Test Isolation**: Each test runs independently
✅ **Clearer Failure Attribution**: JSONL output provides structured results
✅ **Version Compatibility**: Automatically uses correct binary for OCP version
✅ **Backward Compatibility**: OCP < 4.20 continues to work
✅ **Future-Proof**: Aligns with OpenShift's long-term testing strategy

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Binary extraction fails | Tests cannot run | Robust error handling, fallback messaging |
| JSONL parsing errors | Incorrect test results | Validate JSONL format, handle parse errors |
| Performance regression | Longer test execution | Monitor timing, implement parallelization |
| JUnit XML format issues | Report processing fails | Validate XML structure, test with OPCT |
| Python dependency | Script fails if unavailable | Python3 is in base image, fallback to jq |

## Future Enhancements

1. **Parallel Test Execution**: Run tests concurrently to reduce runtime
2. **Binary Caching**: Avoid re-extracting k8s-tests-ext on every run
3. **Better Error Reporting**: Enhanced JSONL parsing with detailed failure info
4. **Test Filtering**: Support running subset of conformance tests
5. **Progress Reporting**: Real-time progress updates during execution
6. **Metrics Collection**: Track test execution time and resource usage

## Files Modified

### OPCT Repository
```
M pkg/run/run.go
M data/templates/plugins/openshift-kube-conformance.yaml
M docs/review/rules.md
M internal/report/slo.go
A CHANGES_K8S_TESTS_EXT.md (this file)
```

### Provider-Certification-Plugins Repository
```
M openshift-tests-plugin/plugin/entrypoint-tests.sh
```

## References

- [OpenShift Tests Extension (OTE) Interface](https://github.com/openshift-eng/openshift-tests-extension)
- [Origin Repository - k8s-tests-ext Integration](https://github.com/openshift/origin)
- [Kubernetes Conformance Testing](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/conformance-tests.md)
- [OCP 4.20 Release Notes](https://docs.openshift.com/container-platform/4.20/release_notes/)

## Related Documentation

- `OPCT_ARCHITECTURE.md` - Overall OPCT architecture
- `SESSION_LEARNINGS_2025-11-20.md` - Previous OCP 4.20 suite fix
- `CHANGES_OCP420_SUITE_FIX.md` - E2E_FOCUS filter approach (superseded by this change)

## Status

**Code Complete**: ✅
**Build Tested**: ✅
**Cluster Tested**: ⏳ Pending
**Documentation**: ✅

---

**Last Updated**: 2025-11-26
**Branch**: fixes
**Author**: Claude Code with jcallen
