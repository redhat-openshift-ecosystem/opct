# Web UI Developer Agent

You are working on the OPCT web UI report — a Vue.js 2 single-page application that displays OpenShift conformance test results. The report is served by a Go HTTP server with both static file serving and a Claude-powered chat API.

## Architecture Overview

### Frontend (single HTML file)
- **Template**: `data/templates/report/report.html` — Go template with `[[`/`]]` delimiters
- **Styles**: `data/templates/report/report.css`
- **Framework**: Vue.js 2 (CDN), Bootstrap (CDN), axios (CDN)
- **Charts**: Chart.js 4.x + chartjs-plugin-zoom (CDN)
- **Markdown**: marked.js (CDN) — used by the chat widget

### Backend
- **Report server**: `pkg/cmd/report/report.go` — Go HTTP server on port 9090
- **Chat API**: `internal/chat/` — Claude API integration with tool use and SSE streaming
- **Data generation**: `internal/report/data.go` — renders templates, produces `opct-report.json`
- **Metrics**: `internal/openshift/mustgathermetrics/` — generates Plotly-format JSON chart data

### Data flow
1. `opct report -s <dir> <archive>` extracts results, generates JSON, renders HTML templates
2. Go HTTP server serves `<dir>/` as static files + mounts `/api/v1/chat/*` handlers
3. Vue app fetches `opct-report.json` via axios, renders pages via `v-html` into `menuBody`
4. Chat widget uses `fetch()` with SSE streaming to `/api/v1/chat`

## Critical Rules

### Layout isolation
- Pages that need multi-column layouts (e.g., etcd split-pane) MUST use `v-if`/`v-else` to conditionally render the split container
- NEVER add `d-flex` or width constraints to a shared container — it breaks all other pages
- Clear inline styles in `changeMenuCleanup()` when navigating away

### Vue rendering
- Content is injected as HTML strings via `v-html` — `<script>` tags inside are ignored
- Use `this.$nextTick(() => { ... })` to run code after DOM update
- Chart.js needs `<canvas>` elements (not `<div>`)
- Always clean up state in `changeMenuCleanup()` when adding new data properties

### Floating widget states
- Use CSS classes with `!important` for state changes (minimize/maximize)
- NEVER toggle `display` on individual children via JS — causes partial blank states
- Three states: normal (default CSS), minimized (`chat-minimized` class), maximized (`chat-maximized` class)

### Chat backend
- Vertex AI: prefer `GOOGLE_CLOUD_LOCATION` over `CLOUD_ML_REGION` (which may be `global`)
- Model IDs: use alias form like `claude-sonnet-4-5` (not dated versions)
- Tool results are read from the report directory filesystem — tools must handle missing files gracefully
- SSE events: `text` (streaming tokens), `tool_call` (tool invocation), `done` (final text), `error`

### AI Attribution
All commits include `Co-Authored-By: Claude <noreply@anthropic.com>`. All GitHub comments end with `— AI Claude`.

### Testing workflow
- ALWAYS rebuild (`make build`) after Go changes
- ALWAYS regenerate report (`rm -rf <dir> && opct report -s <dir> ...`) after template changes
- Test chat with Go server (not python) — python can't serve the API endpoints
- Check at least 3 pages (Summary, Checks, etcd) after any layout change

## File Reference

| Area | File | Purpose |
|------|------|---------|
| Frontend | `data/templates/report/report.html` | Vue.js SPA template (~1600 lines) |
| Styles | `data/templates/report/report.css` | All CSS including split-pane, charts, chat widget |
| Report server | `pkg/cmd/report/report.go` | CLI command, HTTP server setup, chat route registration |
| Chat handler | `internal/chat/handler.go` | HTTP handlers, SSE streaming, Claude API loop |
| Chat tools | `internal/chat/tools.go` | Tool definitions and execution against report data |
| Chat sessions | `internal/chat/session.go` | Session CRUD, JSON persistence |
| Chat prompt | `internal/chat/prompt.go` | System prompt with file override |
| Report data | `internal/report/data.go` | Report struct, template rendering, JSON serialization |
| Metrics gen | `internal/openshift/mustgathermetrics/plotly.go` | Plotly JSON chart data generation |

## CDN Libraries (loaded in report.html head)

| Library | Version | Purpose |
|---------|---------|---------|
| Vue.js | 2.x | Frontend framework |
| Bootstrap | latest | Layout and components |
| BootstrapVue | latest | Vue Bootstrap components |
| axios | latest | HTTP client |
| Chart.js | 4.4.9 | Charts (etcd page) |
| chartjs-adapter-date-fns | 3.0.0 | Time-series X axis |
| Hammer.js | 2.0.8 | Touch gestures for zoom |
| chartjs-plugin-zoom | 2.2.0 | Drag-to-zoom, pan |
| marked.js | 15.0.7 | Markdown rendering (chat) |

## Common Tasks

### Add a new page/menu item
1. Add button in the left nav menu (around line 43-78)
2. Add case in `changeMenu()` switch (around line 267-306)
3. Create `changeMenuNewPage()` method that builds `this.menuBody`
4. Use `this.pageHeadline` at the top for consistent styling

### Add a new chart
1. Define chart metadata in Vue `data()` with `id`, `path`, `title`
2. Build `<canvas>` placeholders in the target panel
3. Fetch JSON via `axios.get()` in `$nextTick()` callback
4. Create Chart.js instance with `responsive: true`
5. Map Plotly JSON format (`data[].x`, `data[].y`, `data[].name`) to Chart.js datasets

### Add a new chat tool
1. Add tool definition in `internal/chat/tools.go` `ToolDefinitions()` function
2. Add input struct if needed (with jsonschema tags)
3. Add case in `Execute()` switch
4. Implement the execution method reading from `te.reportDir`
5. Tool results are returned as JSON strings to Claude

### Modify the chat system prompt
1. Edit `DefaultSystemPrompt` in `internal/chat/prompt.go`
2. Or place a `system.prompt.txt` file in the report directory for override
