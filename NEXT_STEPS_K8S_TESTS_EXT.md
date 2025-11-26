# Next Steps: k8s-tests-ext Integration

## Date: 2025-11-26

## Summary

Successfully implemented k8s-tests-ext support for Kubernetes conformance testing on OCP 4.20+. Code is complete and builds successfully. Ready for testing on a live cluster.

## What Was Done

### OPCT Repository
- ✅ Added `UseK8sTestsExt` flag to detect OCP 4.20+
- ✅ Removed E2E_FOCUS filter approach
- ✅ Added extraction init container to plugin template
- ✅ Updated documentation (rules.md, slo.go)
- ✅ Build tested successfully

### Provider-Certification-Plugins Repository
- ✅ Updated entrypoint-tests.sh for k8s-tests-ext detection
- ✅ Implemented OTE interface for test discovery
- ✅ Implemented OTE interface for test execution
- ✅ Syntax validated, shellcheck passed (warnings acceptable)

## Testing Status

| Test Phase | Status | Priority |
|------------|--------|----------|
| Build OPCT | ✅ Passed | - |
| Build Plugin | ⏳ Pending | High |
| Extract Binary | ⏳ Pending | High |
| Test Discovery | ⏳ Pending | High |
| Test Execution | ⏳ Pending | High |
| Results Processing | ⏳ Pending | Medium |
| OCP 4.19 Compatibility | ⏳ Pending | Medium |
| Full E2E Test | ⏳ Pending | High |

## Immediate Next Steps

### 1. Build Plugin Images

```bash
cd ~/Development/provider-certification-plugins
make build-plugin-tests

# Note the image tag, e.g.:
# quay.io/opct/plugin-openshift-tests:v0.0.0-devel-<commit>
```

### 2. Test on OCP 4.20+ Cluster

#### A. Quick Smoke Test

```bash
cd ~/Development/opct

# Run with custom plugin image
./build/opct-linux-amd64 run \
  --plugins-image=quay.io/opct/plugin-openshift-tests:<tag-from-step-1> \
  --devel-skip-checks

# Monitor init container
kubectl logs -n opct -l sonobuoy-plugin=10-openshift-kube-conformance \
  -c extract-k8s-tests --follow

# Expected: "k8s-tests-ext extracted successfully"
```

#### B. Verify Binary Extraction

```bash
# Wait for pod to be running
kubectl wait --for=condition=Ready pod \
  -l sonobuoy-plugin=10-openshift-kube-conformance -n opct --timeout=5m

# Check binary exists and is executable
kubectl exec -n opct \
  $(kubectl get pod -n opct -l sonobuoy-plugin=10-openshift-kube-conformance -o name) \
  -c tests -- ls -la /tmp/shared/k8s-tests-ext

# Expected: -rwxr-xr-x ... /tmp/shared/k8s-tests-ext

# Test binary works
kubectl exec -n opct \
  $(kubectl get pod -n opct -l sonobuoy-plugin=10-openshift-kube-conformance -o name) \
  -c tests -- /tmp/shared/k8s-tests-ext info

# Expected: JSON output with extension info
```

#### C. Monitor Test Discovery

```bash
# Watch test discovery logs
kubectl logs -n opct \
  $(kubectl get pod -n opct -l sonobuoy-plugin=10-openshift-kube-conformance -o name) \
  -c tests --follow | grep -A 5 "Gathering test list"

# Expected:
# Using k8s-tests-ext binary with OTE interface
# Gathering test list using OTE interface
# Extracted 350 tests from OTE interface
```

#### D. Monitor Test Execution

```bash
# Watch test execution (will take time)
kubectl logs -n opct \
  $(kubectl get pod -n opct -l sonobuoy-plugin=10-openshift-kube-conformance -o name) \
  -c tests --follow

# Expected:
# Using OTE interface - generating and executing k8s-tests-ext start script
# Executing k8s-tests-ext conformance tests...
# Running test: [sig-apps] Deployment...
# Running test: [sig-api-machinery] Aggregator...
# ...
```

#### E. Verify Results

```bash
# Wait for completion (or cancel with Ctrl+C and check partial results)
./build/opct-linux-amd64 retrieve ./results

# Generate report
./build/opct-linux-amd64 report ./results/sonobuoy_*.tar.gz

# Check OPCT-001 status
# Look for: "✓ OPCT-001: Kubernetes Conformance [10-openshift-kube-conformance] must pass 100%"
```

### 3. Test Backward Compatibility (OCP 4.19)

```bash
# On OCP 4.19 cluster
./build/opct-linux-amd64 run --devel-skip-checks

# Verify logs show:
kubectl logs -n opct \
  $(kubectl get pod -n opct -l sonobuoy-plugin=10-openshift-kube-conformance -o name) \
  -c tests | head -50

# Expected:
# Using openshift-tests binary
# (No "extract-k8s-tests" init container)
# Using kubernetes/conformance suite with openshift-tests for OCP 4.19
```

## Potential Issues and Solutions

### Issue 1: Init Container Fails to Extract Binary

**Symptoms**: Init container in CrashLoopBackOff or Error state

**Debug**:
```bash
kubectl logs -n opct <pod-name> -c extract-k8s-tests
```

**Common Causes**:
- Network/registry access issues
- Pull secret not configured
- Hyperkube image not in release (unlikely for OCP 4.20+)

**Solution**:
- Check cluster can access release registry
- Verify cluster version: `oc get clusterversion`
- Check init container has proper RBAC to access ClusterVersion

### Issue 2: Tests All Fail

**Symptoms**: Test execution complete: 0 passed, 350 failed

**Debug**:
```bash
# Check individual test output
kubectl exec -n opct <pod-name> -c tests -- cat /tmp/shared/junit/junit_runner.xml

# Manually run a test
kubectl exec -n opct <pod-name> -c tests -- \
  /tmp/shared/k8s-tests-ext run-test \
  -n "[sig-apps] Deployment deployment should support proportional scaling" \
  -o jsonl
```

**Common Causes**:
- JSONL parsing failure
- KUBECONFIG not set correctly
- TEST_PROVIDER environment variable issue

**Solution**:
- Check python3 available: `kubectl exec ... -- python3 --version`
- Verify KUBECONFIG: `kubectl exec ... -- env | grep KUBECONFIG`
- Test JSONL parsing manually

### Issue 3: Performance Too Slow

**Symptoms**: Test execution takes 4+ hours (expected: 2-3 hours)

**Observation**: k8s-tests-ext runs each test individually, which may add overhead

**Mitigation**:
- Monitor progress to ensure tests are actually running
- Check for network issues or slow cluster response
- Consider parallel execution in future enhancement

**Workaround** (for testing only):
```bash
# Limit number of tests
./opct run --devel-limit-tests=50
```

### Issue 4: JUnit XML Format Invalid

**Symptoms**: OPCT report command fails to parse results

**Debug**:
```bash
# Extract results and check XML
tar xzf results/sonobuoy_*.tar.gz
xmllint --noout plugins/10-openshift-kube-conformance/results/global/junit_runner.xml

# Or check directly in pod
kubectl exec -n opct <pod-name> -c tests -- \
  cat /tmp/shared/junit/junit_runner.xml | head -50
```

**Solution**:
- Validate XML structure
- Check for unescaped special characters in test names
- Review JUnit generation code in entrypoint-tests.sh

## Success Criteria

Before considering this feature production-ready:

- [ ] Binary extraction succeeds on OCP 4.20+
- [ ] Test discovery finds 300+ tests
- [ ] At least 95% of tests pass (assuming healthy cluster)
- [ ] JUnit XML is valid and parsable by OPCT
- [ ] OPCT-001 check evaluates correctly
- [ ] Backward compatibility verified on OCP 4.19
- [ ] End-to-end test completes within reasonable time (< 4 hours)
- [ ] Results match expected conformance test outcomes

## Post-Testing Actions

### If Testing Succeeds

1. **Commit changes**:
   ```bash
   cd ~/Development/opct
   git add -A
   git commit -m "feat: Switch to k8s-tests-ext for K8s conformance on OCP 4.20+

   - Extract k8s-tests-ext from hyperkube image at runtime
   - Implement OTE interface for test discovery and execution
   - Generate JUnit XML from JSONL results
   - Maintain backward compatibility for OCP < 4.20

   Resolves kubernetes/conformance suite issues on OCP 4.20+"

   cd ~/Development/provider-certification-plugins
   git add -A
   git commit -m "feat: Add k8s-tests-ext support with OTE interface

   - Detect k8s-tests-ext binary and use OTE interface
   - Implement test discovery via k8s-tests-ext list
   - Implement test execution via k8s-tests-ext run-test
   - Parse JSONL output and generate JUnit XML
   - Maintain backward compatibility with openshift-tests"
   ```

2. **Create pull requests**:
   - OPCT repository PR
   - Provider-certification-plugins repository PR

3. **Update documentation**:
   - Link to new docs in main README
   - Update user guides with OCP 4.20+ information

### If Testing Reveals Issues

1. **Document findings** in issue tracker
2. **Iterate on fixes** based on test results
3. **Re-test** after fixes
4. **Consider rollback plan** if issues are severe

## Long-Term Enhancements

After initial release:

1. **Performance Optimization**:
   - Implement parallel test execution
   - Reduce per-test overhead
   - Cache k8s-tests-ext binary

2. **Better Observability**:
   - Real-time progress reporting
   - Test timing metrics
   - Resource usage tracking

3. **Enhanced Error Handling**:
   - Retry failed tests
   - Better error messages in JUnit XML
   - Automatic fallback mechanisms

4. **Test Filtering**:
   - Run subset of conformance tests
   - Skip known failures
   - Custom test selection

## Documentation Files

- **CHANGES_K8S_TESTS_EXT.md** - Complete technical documentation (OPCT)
- **CHANGES_K8S_TESTS_EXT_SUPPORT.md** - Plugin implementation details
- **NEXT_STEPS_K8S_TESTS_EXT.md** - This file

## Contact

For questions or issues:
- File issue in OPCT repository
- Tag: `enhancement`, `kubernetes-conformance`, `ocp-4.20`

---

**Status**: Ready for Testing
**Last Updated**: 2025-11-26
