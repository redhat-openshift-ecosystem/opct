# OPCT Developer Agent

You are working on OPCT (OpenShift Platform Compatibility Tool) — a Go CLI that orchestrates conformance test workflows on OpenShift clusters and generates web UI reports.

## Project Structure

```
cmd/opct/              CLI entrypoint (cobra commands)
pkg/                   Public packages (cmd handlers, client, types, run, status, wait)
internal/              Internal packages (report, chat, assets, summary, metrics, mustgather)
data/templates/        Embedded templates (report HTML/CSS, plugin manifests)
docs/                  Documentation (user guides, developer guides, review docs)
hack/                  Build scripts, Containerfile
.github/workflows/     CI/CD (go.yaml, pre_linters.yaml, pre_reviewer.yaml, e2e.yaml)
```

## Build and Validate

Always run these before committing:

```bash
go mod tidy          # resolve dependencies
make build           # build binary to build/opct-linux-amd64
make test            # run unit tests
make vet             # run go vet
make test-lint       # lint (YAML workflows + Go)
```

If `make test-lint` fails, verify that each failure predates your change. Do not ignore failures in modified workflow files.

## Key Conventions

### Commits
- Follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`
- Always include the AI sign-off (see below)

### AI Attribution (Required)

**Git commits:** include `Co-Authored-By: Claude <noreply@anthropic.com>` trailer.

**GitHub interactions** (PR descriptions, comments, review replies): append `— AI Claude` at the end.

**Jira comments** (via Claude Code): append `— AI Claude` at the end. The Chai Bot VCSP persona uses its own built-in attribution — see CLAUDE.md "AI Agent Ecosystem".

### Error Handling
- Use `fmt.Errorf("context: %w", err)` for wrapping
- Check error returns on `json.Encode`, `fmt.Fprintf`, and similar I/O calls
- Use `log` (logrus) for logging: `log.Infof`, `log.Warnf`, `log.Errorf`, `log.Debugf`

### Dependencies
- OpenShift client libs: `github.com/openshift/api@release-X.Y`, `github.com/openshift/client-go@release-X.Y`
- Kubernetes client libs: `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go` (version `v0.X.Y` maps to k8s `v1.X.Y`)
- Anthropic SDK: `github.com/anthropics/anthropic-sdk-go` (with `vertex` subpackage)
- Do NOT add `toolchain` directive to go.mod (CI compatibility issue)

### Release Process
**Use the `opct-release` skill** (`.claude/skills/opct-release/`) — authoritative runbook for both RC (`vX.Y.Z-rc.N`) and GA (`vX.Y.Z`) releases, covering both repos, the `openshift/release` CI bump, release-page fixes and failed-tag recovery.

Invariants worth knowing before you read it:
- Manual annotated tags pinned to an explicit SHA. No release automation.
- Release plugins BEFORE the CLI — the `verify-images` job checks that the three plugin tags pinned in `pkg/types.go` (`PluginsImage`, `CollectorImage`, `MustGatherMonitoringImage`) already exist on quay. `ControllerImage` is not existence-checked.
- Release branches are `release-X.Y` in **both** repos (no `v` prefix): `release-0.7`, not `release-v0.7`.
- `main` always carries `:latest` in `pkg/types.go`; pinning happens only on release branches, where CI blocks `:latest`.
- Merge into release branches, never rebase — a rebase orphans previously tagged commits.
- Bumping `OPCT_CLI_IMAGE` in `openshift/release` is part of the release, not a follow-up.

## Important Files

| File | Purpose |
|------|---------|
| `pkg/types.go` | Plugin image version constants (update for releases) |
| `pkg/run/run.go` | `opct run` command — pre-run validations, environment setup |
| `pkg/cmd/report/report.go` | `opct report` command — report generation and HTTP server |
| `internal/report/data.go` | Report data structures and template rendering |
| `internal/chat/` | AI assistant chatbot (handler, tools, sessions, prompt) |
| `CLAUDE.md` | Full development instructions (Go bumps, deps, release, web UI) |
| `internal/cleaner/` | Archive cleaner: JSON patches, file removal, leak scanning |

## Testing OPCT Report Changes

See the `webui-report-test` skill for the build-regenerate-serve workflow.
Key: always clear the report dir before regenerating to pick up template changes:
```bash
REPORT_DIR=~/opct/tmp/${TEST_ID}__report
REPORT_ROOT="${HOME}/opct/tmp"
REAL_DIR="$(realpath -m "${REPORT_DIR}")"
case "${REAL_DIR}" in
  "${REPORT_ROOT}"/*) ;;
  *) echo "refusing to delete outside ${REPORT_ROOT}: ${REAL_DIR}" >&2; exit 1 ;;
esac
[ -L "${REPORT_DIR}" ] && { echo "refusing to delete symlink: ${REPORT_DIR}" >&2; exit 1; }
rm -rf "${REPORT_DIR:?}"/*
```

## Related Skills

- **`opct-release`** — RC and GA release runbook (both repos, openshift/release bump, release page, recovery). Use for anything release-related.
- **`opct-runtime`** — Plugin runtime architecture (execution order, dependency chain, timing delays). Use when investigating plugin timing, startup delays, or dependency issues.
- **`jira-ops`** — Jira operations with MCP-first + REST API fallback. Use when filing bugs or comments.
- **`ci-triage`** — CI job failure triage workflow. Use when investigating periodic job failures.
