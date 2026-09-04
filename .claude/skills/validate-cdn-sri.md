---
name: validate-cdn-sri
description: Validate CDN dependencies for security (whitelist, pinning, SRI hashes)
---

# Validate CDN Security

Run this before committing changes to HTML files to validate CDN security compliance.

## Quick Start

```bash
bash hack/check-cdn-sri.sh
```

## What It Checks

1. **CDN Whitelist** - Only `cdn.jsdelivr.net` and `unpkg.com` allowed
2. **Version Pinning** - No `@latest`, exact semver for Vue.js (e.g., `@2.7.14`)
3. **SRI Hashes** - All CDN dependencies must have `integrity="sha384-..."` 
4. **SRI Format** - Must be valid SHA384 Base64 encoded

## Files Validated

- `data/templates/report/report.html`
- `data/templates/report/filter.html`

## Output

**Success:**
```
✅ CHECK PASSED
```

**Failure:**
```
❌ CHECK FAILED
```

With detailed information about what needs to be fixed.

## When to Use

Run this before pushing when you've modified any HTML files with CDN dependencies:

```bash
# 1. Make your changes
# 2. Validate locally
bash hack/check-cdn-sri.sh

# 3. If it passes, commit and push
# 4. If it fails, fix the issues and re-run
```

## Fixing Issues

### Missing or Invalid SRI Hash

1. Get the CDN URL
2. Generate SRI hash:
   ```bash
   curl -s <CDN_URL> | openssl dgst -sha384 -binary | openssl base64 -A
   ```
3. Add to HTML element:
   ```html
   <script src="<CDN_URL>"
           integrity="sha384-<PASTE_HASH_HERE>"
           crossorigin="anonymous"></script>
   ```

### Unpinned Version (@latest)

Change from:
```html
<script src="https://unpkg.com/package@latest/..."></script>
```

To:
```html
<script src="https://unpkg.com/package@1.2.3/..."></script>
```

### Vue.js Version Not Semver

Change from:
```html
<script src="https://cdn.jsdelivr.net/npm/vue@2/..."></script>
```

To:
```html
<script src="https://cdn.jsdelivr.net/npm/vue@2.7.14/..."></script>
```

### Unauthorized CDN

Only these are allowed:
- `cdn.jsdelivr.net`
- `unpkg.com`

Change to use one of these instead.

## CI Check

This same check runs in GitHub Actions on every PR that modifies HTML files.

The local script lets you validate before pushing, avoiding CI failures.
