---
name: webui-chat-debug
description: Debug the OPCT chatbot backend — test API endpoints, check auth, inspect tool execution, and troubleshoot Vertex/Anthropic API issues.
allowed-tools: Bash, Read
---

# Web UI Chat Debug

Debug the OPCT chatbot backend when chat isn't working or producing errors.

## When to use

- Chat shows "not available" or API errors
- Vertex AI or Anthropic API authentication issues
- Tool execution returning wrong data
- SSE streaming not working
- Session save/load failures

## Diagnostic steps

### 1. Check chat status

```bash
curl -s http://localhost:9090/api/v1/chat/status | python3 -m json.tool
```

Expected: `{"enabled": true, "provider": "vertex"|"anthropic", "model": "claude-sonnet-4-5"}`

If `enabled: false`:
- For Vertex AI: `echo $GOOGLE_CLOUD_LOCATION $ANTHROPIC_VERTEX_PROJECT_ID` (both must be set)
- Fallbacks also checked: `CLOUD_ML_REGION`, `GOOGLE_CLOUD_PROJECT`
- For Anthropic API: `echo $ANTHROPIC_API_KEY`

### 2. Test a simple chat message

```bash
curl -s -X POST http://localhost:9090/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "hello", "history": []}' 2>&1 | head -20
```

Should stream SSE events like:
```
event: text
data: Hello...
```

### 3. Check tool execution directly

Tools read from the report directory. Verify the report data exists:

```bash
TEST_ID=4.20.0-0.nightly-2026-05-04-230007-20260510-HighlyAvailable-aws-External
ls ~/opct/tmp/${TEST_ID}__report/opct-report.json
ls ~/opct/tmp/${TEST_ID}__report/failures-*/
ls ~/opct/tmp/${TEST_ID}__report/metrics/
```

### 4. Check sessions directory

```bash
curl -s http://localhost:9090/api/v1/chat/sessions | python3 -m json.tool
ls ~/opct/tmp/${TEST_ID}__report/chat-sessions/
```

### 5. Common errors

| Error | Cause | Fix |
|-------|-------|-----|
| `404 Not Found` from Vertex | Wrong model ID or region | Use alias `claude-sonnet-4-5` (not dated). Use `GOOGLE_CLOUD_LOCATION` not `CLOUD_ML_REGION=global` |
| `enabled: false` | No API credentials detected | Set `GOOGLE_CLOUD_LOCATION` + `ANTHROPIC_VERTEX_PROJECT_ID` (Vertex) OR `ANTHROPIC_API_KEY` (direct) |
| Chat panel shows "unable to connect" | Server not running or wrong port | Start with `opct report -s <dir> <archive>` (not `--skip-server`) |
| Tool returns empty data | Report JSON missing or malformed | Regenerate report: `rm -rf <dir> && opct report -s <dir> --skip-server <archive>` |

## Key files

- `internal/chat/handler.go` — HTTP handlers, auth detection, streaming loop
- `internal/chat/tools.go` — Tool definitions and execution against report dir
- `internal/chat/prompt.go` — System prompt (built-in + file override)
- `internal/chat/session.go` — Session persistence
- `pkg/cmd/report/report.go` — Route registration (lines ~196-204)
