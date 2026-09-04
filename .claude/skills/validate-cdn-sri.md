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
2. **Version Pinning** - No `@latest`; every CDN package must use exact `@X.Y.Z` semver
3. **SRI Hashes** - All CDN dependencies must have `integrity="sha384-..."`
4. **SRI Format** - Must be a non-empty SHA-384 Base64 digest (`sha384-` + 64 chars)

## Files Validated

- `data/templates/report/report.html`
- `data/templates/report/filter.html`
- `internal/openshift/mustgathermetrics/metrics.html`

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

### Partial or Missing Package Version

Every CDN package must use exact `@X.Y.Z` semver (not `@1`, `@4`, `@2`, or unversioned URLs).

Change from:
```html
<script src="https://unpkg.com/axios/dist/axios.min.js"></script>
<script src="https://unpkg.com/bootstrap@4/dist/css/bootstrap.min.css"></script>
<script src="https://cdn.jsdelivr.net/npm/vue@2/dist/vue.js"></script>
```

To:
```html
<script src="https://unpkg.com/axios@1.20.0/dist/axios.min.js"></script>
<link href="https://unpkg.com/bootstrap@5.3.8/dist/css/bootstrap.min.css">
<script src="https://cdn.jsdelivr.net/npm/vue@2.7.14/dist/vue.js"></script>
```

### Unauthorized CDN

Only these are allowed:
- `cdn.jsdelivr.net`
- `unpkg.com`

Change to use one of these instead.

## Adversarial Fixture Tests

The validator also runs regression fixtures under `test/testdata/cdn-sri/`:

```bash
bash hack/check-cdn-sri.sh --test-fixtures
```

Production validation plus fixtures run automatically in the default mode:

```bash
bash hack/check-cdn-sri.sh
```

This same check runs in GitHub Actions on every PR that modifies HTML files.

The local script lets you validate before pushing, avoiding CI failures.
