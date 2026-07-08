---
name: opct-runtime
description: OPCT plugin runtime architecture — execution order, dependency chain, timing delays, and key files for investigating plugin and workflow issues.
allowed-tools: Bash, Read
---

# OPCT Plugin Runtime Architecture

Reference knowledge for investigating plugin timing, dependency, and workflow issues. Used by `ci-triage` and `opct-developer` agents.

## Plugin Execution Order

Plugins run as sonobuoy Job pods, all created simultaneously. Dependencies are managed inside the plugin containers, not by the scheduler.

```
05-openshift-cluster-upgrade  →  10-openshift-kube-conformance  →  20-openshift-conformance-validated  →  80-openshift-tests-replay  →  99-openshift-artifacts-collector
```

Each plugin waits for its predecessor to complete before starting test execution.

## Dependency Chain Components

Four components add sequential delays between plugin completion and successor startup:

| Component | Delay | Location |
|-----------|-------|----------|
| sonobuoy-worker done-file sleep | 5s (hardcoded) | sonobuoy upstream (not configurable) |
| Aggregator annotation update | 5s + 1.2x jitter | sonobuoy upstream `run.go:45` |
| Plugin dependency waiter poll | 10s | `plugin.go:683` in plugins repo |
| Tests container start-file poll | 10s | `entrypoint-tests.sh:165` in plugins repo |
| openshift-tests init (extension discovery) | ~3 min | upstream openshift-tests |

Worst case additive delay after predecessor finishes: ~30s polling + ~3min openshift-tests init.

Improvement tracked in [OPCT-409](https://issues.redhat.com/browse/OPCT-409).

## How Dependencies Work

### Plugin container (`plugin` sidecar)

1. Waits for suite list file (`/tmp/shared/suite.list.done`)
2. Starts dependency waiter — polls sonobuoy aggregator annotation (`sonobuoy.hept.io/status`) every 10 seconds via `sbaggregation.GetStatus()` which reads the sonobuoy pod annotation
3. When predecessor shows `status=complete` or `podPhase=Completed`, writes `/tmp/shared/start`
4. Monitors test progress via `/tmp/shared/fifo` and reports to aggregator via progress API

### Tests container (`tests`)

1. Runs `openshift-tests` to list suite (generates `/tmp/shared/suite.list`)
2. Polls for `/tmp/shared/start` file every 10 seconds (bash `sleep 10` loop)
3. When start file appears, runs `openshift-tests run` with the suite list

### Sonobuoy aggregator (`sonobuoy` pod)

1. Receives progress updates from plugin pods via HTTP POST to `/api/v1/progress/`
2. Receives results via HTTP PUT to `/api/v1/results/`
3. Updates pod annotation (`sonobuoy.hept.io/status`) every 5s via `wait.JitterUntil()`
4. The annotation is what both `opct status` (client) and dependency waiters (plugin) read

### Sonobuoy worker (`sonobuoy-worker` sidecar in each plugin pod)

1. Watches for done file at `/tmp/sonobuoy/results/done`
2. **Sleeps 5 seconds** after detecting done file ("allows other containers to intervene")
3. Uploads result tarball to aggregator via PUT

## Status Messages in `opct status`

| Message Pattern | Meaning |
|----------------|---------|
| `status=blocked-by={PLUGIN}` | Predecessor's predecessor is still running |
| `status=waiting-for={PLUGIN}=(0/{N}/0)` | Predecessor is running, {N} tests remaining |
| `status=running=T/C/P/F/S={total}/{completed}/{passed}/{failed}/{skipped}` | Plugin is actively running tests |
| `waiting for jobs initialization=PodStatus(NotReady)` | Plugin pod containers still starting |

## Runtime Data in Report

The `meta/run.log` file in the sonobuoy archive contains JSON-formatted server events. Parsed by `internal/opct/archive/metalog.go`:

- `"plugin started {NAME}"` — when plugin sends first POST (progress update)
- `"plugin finished {NAME}"` — when plugin sends PUT (result upload)
- `"server finished"` — when aggregator cleanup runs

Delta times computed as `finish[current] - finish[predecessor]`. If predecessor never finished, falls back to plugin's own start time (fixed in OPCT-408).

## Key Files

### OPCT CLI repo (`opct`)

| File | Purpose |
|------|---------|
| `internal/opct/archive/metalog.go` | Parses `meta/run.log` for runtime data |
| `pkg/status/printer.go` | `opct status` output formatting |
| `data/templates/plugins/*.yaml` | Plugin manifest templates |

### Plugins repo (`provider-certification-plugins`)

| File | Purpose |
|------|---------|
| `openshift-tests-plugin/pkg/plugin/plugin.go:680-820` | Dependency waiter loop |
| `openshift-tests-plugin/pkg/plugin/blocker.go` | `GetPluginsBlocker()` — reads sonobuoy annotation |
| `openshift-tests-plugin/plugin/entrypoint-tests.sh:154-166` | Start-file polling loop |

## Investigating Plugin Timing Issues

1. Check `plugin` container logs: `oc logs {pod} -c plugin -n opct` — shows dependency waiter reconcile cycles
2. Check `tests` container logs: `oc logs {pod} -c tests -n opct` — shows start-file polling and openshift-tests init
3. Check `sonobuoy-worker` container logs: `oc logs {pod} -c sonobuoy-worker -n opct` — shows done-file detection and result upload
4. Check aggregator logs: `oc logs sonobuoy -n opct` — shows PUT/POST requests from plugins
5. Check annotation directly: `oc get pod sonobuoy -n opct -o jsonpath='{.metadata.annotations.sonobuoy\.hept\.io/status}'`
