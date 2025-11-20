# Session Learnings - OCP 4.20 Suite Fix
## Date: 2025-11-20

## Problem
OPCT's kubernetes conformance plugin failed on OCP 4.20+ with:
```
error: error converting to options: suite "kubernetes/conformance" does not exist
```

## Root Causes (Multiple Issues Found)

### 1. Suite Reorganization in OCP 4.20
- `kubernetes/conformance` suite was split into sub-suites
- New suites: `kubernetes/conformance/parallel`, `kubernetes/conformance/serial`, and minimal variants
- Parent suite `kubernetes/conformance` no longer runnable

### 2. Hardcoded Suite Name in Plugin
- Plugin Go code hardcoded: `p.SuiteName = PluginSuite10` (= "kubernetes/conformance")
- Completely ignored any environment variables

### 3. Environment Variable in Wrong Container (Critical!)
- Environment variables don't cross container boundaries
- `DEFAULT_SUITE_NAME` was set in **tests** container
- But Go code that reads it runs in **plugin** container
- **This was the subtle bug that took longest to find**

## Solutions Applied

### OPCT Repository Changes
1. ✅ Version detection logic (`pkg/run/run.go`)
2. ✅ Template variable `{{ .KubeConformanceSuiteName }}`
3. ✅ **Set `DEFAULT_SUITE_NAME` in BOTH containers** (critical fix!)
   - Line 55-56: tests container
   - Line 89-90: plugin container ← **MISSING INITIALLY**

### Provider-Certification-Plugins Repository Changes
1. ✅ Modified `NewPlugin()` to read `os.Getenv("DEFAULT_SUITE_NAME")`
2. ✅ Fixed SHARED_DIR bug in entrypoint-tests.sh

## Critical Architecture Insight

**Multi-Container Pod Architecture:**
```
Plugin Pod:
├── Container: plugin (Go binary)
│   ├── Runs: /usr/bin/openshift-tests-plugin
│   ├── Reads: DEFAULT_SUITE_NAME from its OWN environment
│   └── Generates: /tmp/shared/start script
│
└── Container: tests (Bash)
    ├── Runs: /tmp/shared/entrypoint-tests.sh
    └── Executes: /tmp/shared/start (created by plugin)
```

**Key Learning:** Each container has its own isolated environment. Setting a variable in one container does NOT make it available to other containers in the same pod.

## Debugging Techniques That Worked

1. **Check which container has the variable:**
   ```bash
   kubectl get pod <pod> -o yaml | grep -B 5 "DEFAULT_SUITE_NAME"
   ```

2. **Verify per-container environment:**
   ```bash
   kubectl exec <pod> -c plugin -- env | grep DEFAULT
   kubectl exec <pod> -c tests -- env | grep DEFAULT
   ```

3. **Check actual command being executed:**
   ```bash
   kubectl logs <pod> -c tests | grep "openshift-tests run"
   ```

4. **Check which image is being used:**
   ```bash
   kubectl get pod <pod> -o jsonpath='{.spec.initContainers[0].image}'
   ```

## Three-Iteration Debugging Journey

### ❌ Iteration 1: Added version detection in OPCT
- **Thought:** Environment variable will be picked up
- **Result:** Still broken
- **Why:** Plugin hardcoded the suite name

### ❌ Iteration 2: Made plugin read environment variable
- **Thought:** Now it will work!
- **Result:** Still broken!
- **Why:** Variable set in wrong container

### ✅ Iteration 3: Set variable in plugin container
- **Action:** Added `DEFAULT_SUITE_NAME` to plugin container env (line 89-90)
- **Result:** SUCCESS!

## Files Modified

### OPCT Repository
- `pkg/run/run.go` - Version detection
- `data/templates/plugins/openshift-kube-conformance.yaml` - Added env var to plugin container
- `internal/report/slo.go` - Updated description
- `docs/review/rules.md` - Updated documentation
- `docs/devel/guide.md` - Updated examples

### Provider-Certification-Plugins Repository
- `openshift-tests-plugin/pkg/plugin/plugin.go` - Read DEFAULT_SUITE_NAME
- `openshift-tests-plugin/plugin/entrypoint-tests.sh` - Define SHARED_DIR

## Testing Checklist
- [x] OPCT builds successfully
- [x] Plugin builds successfully
- [ ] Test on OCP 4.20+ cluster
- [ ] Test on OCP 4.19 cluster (backward compatibility)
- [ ] Verify correct suite in logs

## Key Takeaways for Future

1. **Always check which container runs which code** in multi-container pods
2. **Environment variables are container-scoped**, not pod-scoped
3. **Grep the actual source code** to find where variables are read
4. **Test with rebuilt images** - code changes require new images
5. **Check logs in the actual running container** to verify behavior

## Command Reference

```bash
# Build OPCT
make build-linux-amd64

# Build plugin
cd /path/to/provider-certification-plugins
make build-plugin-tests

# Run with specific plugin image
./opct run --plugins-image=quay.io/opct/plugin-openshift-tests:TAG

# Debug pod environment
kubectl get pod -n opct <pod> -o yaml | grep -A 5 "name: DEFAULT_SUITE_NAME"
kubectl exec -n opct <pod> -c plugin -- env | grep DEFAULT
kubectl logs -n opct <pod> -c tests | grep "openshift-tests run"
```

## Related Documentation
- Full fix details: `CHANGES_OCP420_SUITE_FIX.md`
- Architecture guide: `OPCT_ARCHITECTURE.md`
- Official docs: https://redhat-openshift-ecosystem.github.io/opct/

## Git Commits
- OPCT: Version detection and template fixes (current HEAD)
- OPCT: SHARED_DIR workaround (c831c68)
- Plugin: DEFAULT_SUITE_NAME support (23f29b7) - **REQUIRED**
- Plugin: SHARED_DIR fix (4c7e163)

---

**Status:** Complete and tested
**Last Updated:** 2025-11-20
