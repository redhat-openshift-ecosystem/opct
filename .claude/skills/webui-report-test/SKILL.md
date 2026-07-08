---
name: webui-report-test
description: Build, regenerate, and serve the OPCT web UI report for browser testing. Use when modifying report.html, report.css, chat backend, or any template/chart changes.
allowed-tools: Bash, Read
---

# Web UI Report Test

Build the project, regenerate the report from a test archive, and serve it for browser testing.

## When to use

- After modifying `data/templates/report/report.html` or `report.css`
- After changing report data generation in `internal/report/data.go`
- After changing metrics generation in `internal/openshift/mustgathermetrics/`
- After changing chat backend in `internal/chat/`
- When testing any web UI layout, chart, chatbot, or table changes

## Workflow

### 1. Build

```bash
make build
```

### 2. Generate report and serve

There are two modes depending on what you're testing:

**With chat API (use when testing chatbot or full integration):**
```bash
TEST_ID=4.20.0-0.nightly-2026-05-04-230007-20260510-HighlyAvailable-aws-External

# Kill previous server
pkill -f "opct-linux-amd64 report" 2>/dev/null; sleep 1

# Generate and serve (Go HTTP server — serves both static files and chat API)
rm -rf ~/opct/tmp/${TEST_ID}__report
./build/opct-linux-amd64 report -s ~/opct/tmp/${TEST_ID}__report ~/opct/results/${TEST_ID}.tar.gz
```

**Without chat API (use for pure frontend/layout changes):**
```bash
TEST_ID=4.20.0-0.nightly-2026-05-04-230007-20260510-HighlyAvailable-aws-External

# Generate only
rm -rf ~/opct/tmp/${TEST_ID}__report
./build/opct-linux-amd64 report -s ~/opct/tmp/${TEST_ID}__report --skip-server ~/opct/results/${TEST_ID}.tar.gz

# Serve with python (lightweight, no chat API)
pkill -f "python3 -m http.server 9090" 2>/dev/null; sleep 0.5
python3 -m http.server 9090 --directory ~/opct/tmp/${TEST_ID}__report
```

### 3. Test in browser

Open http://localhost:9090 and verify:

**Always check:**
- The page you modified works correctly
- At least 2-3 other pages (Summary, Checks, Network) have no layout regressions
- Browser resize works for responsive layouts

**For chart changes:**
- Charts load and are interactive (zoom, expand)
- etcd split-pane layout works, divider is draggable

**For chatbot changes:**
- Chat bubble appears in bottom-right
- `/api/v1/chat/status` returns enabled status
- Streaming responses work
- Minimize/maximize/close buttons work
- Stop button cancels mid-stream responses
- Sessions auto-save and load correctly

**For layout changes (headline, tables):**
- Verify styling on ALL pages that use `pageHeadline` (Summary, Checks, etcd, Network, Runtime, etc.)

## Important notes

- The Go server (`opct report` without `--skip-server`) blocks the terminal — run in background
- The Go server is REQUIRED for chat API — python server only serves static files
- Chat requires Vertex AI or Anthropic API credentials (see CLAUDE.md for env vars)
- Report generation takes ~15 seconds for the test archive
- Always `rm -rf` the report dir before regenerating to pick up template changes
