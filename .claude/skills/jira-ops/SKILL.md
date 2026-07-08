---
name: jira-ops
description: Jira operations for the OPCT project — MCP-first with REST API fallback for creating issues, comments, and links.
allowed-tools: Bash, mcp__jira__jira_create_issue, mcp__jira__jira_add_comment, mcp__jira__jira_create_issue_link, mcp__jira__jira_search, mcp__jira__jira_update_issue, mcp__jira__jira_get_issue
---

# Jira Operations for OPCT

Shared operational knowledge for Jira interactions in the OPCT project. Used by `ci-triage` and `opct-developer` agents.

## Strategy: MCP first, REST API fallback

Always try MCP tools first. If MCP returns a permission error, fall back to REST API via `curl`.

## Creating issues

**MCP (try first):**
```
mcp__jira__jira_create_issue(
    project_key="OPCT",
    summary="...",
    issue_type="Bug",
    description="<jira wiki markup>",
    additional_fields='{"labels": [...], "parent": {"key": "OPCT-400"}}'
)
```

**REST API fallback:**
```bash
curl -s -X POST -H "Content-Type: application/json" \
  -u "${JIRA_USERNAME}:${JIRA_API_TOKEN}" \
  "https://redhat.atlassian.net/rest/api/2/issue" \
  -d '{"fields": {"project": {"key": "OPCT"}, "issuetype": {"name": "Bug"}, ...}}'
```

If credentials are not exported, ask the user where their Jira credentials file is located.

## Adding comments

**MCP (try first):**
```
mcp__jira__jira_add_comment(issue_key="OPCT-XXX", body="Comment in Markdown")
```

**REST API fallback:**
```bash
curl -s -X POST -H "Content-Type: application/json" \
  -u "${JIRA_USERNAME}:${JIRA_API_TOKEN}" \
  "https://redhat.atlassian.net/rest/api/2/issue/OPCT-XXX/comment" \
  -d '{"body": "Comment in Jira wiki markup"}'
```

## Linking issues

**MCP (try first):**
```
mcp__jira__jira_create_issue_link(link_type="Related", inward_issue_key="OPCT-NEW", outward_issue_key="OPCT-EXISTING")
```

**REST API fallback:**
```bash
curl -s -X POST -H "Content-Type: application/json" \
  -u "${JIRA_USERNAME}:${JIRA_API_TOKEN}" \
  "https://redhat.atlassian.net/rest/api/2/issueLink" \
  -d '{"type": {"name": "Related"}, "inwardIssue": {"key": "OPCT-NEW"}, "outwardIssue": {"key": "OPCT-EXISTING"}}'
```

## Link type reference

| Name | Inward | Outward |
|------|--------|---------|
| `Related` | is related to | relates to |
| `Blocks` | is blocked by | blocks |

**Common mistake:** `"Relates"` returns 404 — the correct name is `"Related"`.

## Important notes

- **API URL:** Always use `https://redhat.atlassian.net` directly. Do NOT use `https://issues.redhat.com` — it returns a 301 redirect that drops the POST body.
- **Description format:** MCP tools accept Markdown. REST API uses Jira wiki markup (`h2.`, `*bold*`, `{noformat}`).
- **All Jira comments/descriptions created by AI** must end with `— AI Claude`.
