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
```

`make test-lint` may show pre-existing YAML lint issues — that's OK if unrelated to your changes.

## Key Conventions

### Commits
- Follow [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`
- Always include the AI sign-off (see below)

### AI Attribution (Required)

**Git commits:** include `Co-Authored-By: Claude <noreply@anthropic.com>` trailer.

**GitHub interactions** (PR descriptions, comments, review replies): append `— AI Claude` at the end.

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
- Manual tag creation (not automated workflows)
- Release plugins BEFORE CLI (CLI references plugin image versions in `pkg/types.go`)
- Release branches: `release-X.Y` (CLI), `release-vX.Y` (plugins)
- See CLAUDE.md "Release Process" section for full procedure

## Important Files

| File | Purpose |
|------|---------|
| `pkg/types.go` | Plugin image version constants (update for releases) |
| `pkg/run/run.go` | `opct run` command — pre-run validations, environment setup |
| `pkg/cmd/report/report.go` | `opct report` command — report generation and HTTP server |
| `internal/report/data.go` | Report data structures and template rendering |
| `internal/chat/` | AI assistant chatbot (handler, tools, sessions, prompt) |
| `CLAUDE.md` | Full development instructions (Go bumps, deps, release, web UI) |

## Testing OPCT Report Changes

See the `webui-report-test` skill for the build-regenerate-serve workflow.
Key: always `rm -rf` the report dir before regenerating to pick up template changes.

## Related Skills

- **`opct-runtime`** — Plugin runtime architecture (execution order, dependency chain, timing delays). Use when investigating plugin timing, startup delays, or dependency issues.
- **`jira-ops`** — Jira operations with MCP-first + REST API fallback. Use when filing bugs or comments.
- **`ci-triage`** — CI job failure triage workflow. Use when investigating periodic job failures.
