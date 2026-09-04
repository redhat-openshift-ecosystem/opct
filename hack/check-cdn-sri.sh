#!/bin/bash
# CDN SRI (Subresource Integrity) verification script
#
# Usage: ./hack/check-cdn-sri.sh
#
# This script validates that all CDN dependencies in HTML templates:
# 1. Use only whitelisted CDN hosts (cdn.jsdelivr.net, unpkg.com)
# 2. Have pinned versions (no @latest, exact semver for Vue.js)
# 3. Include SRI hashes for integrity verification
#
# Run this locally before pushing to verify CDN security compliance.

set -e

echo "=========================================="
echo "CDN SECURITY CHECK"
echo "=========================================="

failed=0

# Check report.html and filter.html
for file in data/templates/report/report.html data/templates/report/filter.html; do
  echo ""
  echo "Checking $file..."

  # Collapse to single line so multiline <script>/<link> elements are parsed correctly
  collapsed=$(tr '\n' ' ' < "$file")

  # Check 1: No unauthorized CDN hosts in <script>/<link> elements
  # URL_CHARS stops at double-quote, single-quote, space, or > — handles all quote styles
  # Anchored hostname match rejects cdn.jsdelivr.net.evil.invalid style bypasses
  URL_CHARS='[^"'"'"' >]'
  bad_cdns=$(echo "$collapsed" | grep -oE '<(script|link)[^>]+>' | \
    grep -oE "https://${URL_CHARS}+" | \
    grep -oE 'https://[^/]+' | \
    grep -Ev '^https://(cdn\.jsdelivr\.net|unpkg\.com)$' || true)
  if [ -n "$bad_cdns" ]; then
    echo "❌ FAIL: Found unauthorized CDN hosts in script/link tags"
    echo "$bad_cdns"
    failed=1
  fi

  # Check 2: No @latest versions (URL patterns are always single-line)
  if grep -q '@latest' "$file"; then
    echo "❌ FAIL: Found @latest (unpinned) versions"
    grep -n '@latest' "$file" || true
    failed=1
  fi

  # Check 3: Vue.js must use exact semver X.Y.Z (rejects @2, @2.7, @2.7.14foo, @2.7.14.1)
  bad_vue=$(grep -oE 'vue@[^/"]+' "$file" | grep -Ev '^vue@[0-9]+\.[0-9]+\.[0-9]+$' || true)
  if [ -n "$bad_vue" ]; then
    echo "❌ FAIL: Vue.js not pinned to exact semver X.Y.Z (use @2.7.14, not @2 or @2.7.14foo)"
    echo "$bad_vue"
    failed=1
  fi

  # Check 4: Every CDN <script>/<link> element must have an integrity attribute
  # Match CDN URLs regardless of quote style (no (src|href)=" prefix required)
  cdn_count=$(echo "$collapsed" | grep -oE '<(script|link)[^>]+>' | \
    grep -cE 'https://(cdn\.jsdelivr\.net|unpkg\.com)' || true)
  sri_count=$(echo "$collapsed" | grep -oE '<(script|link)[^>]+>' | \
    grep -E 'https://(cdn\.jsdelivr\.net|unpkg\.com)' | \
    grep -c 'integrity=' || true)

  if [ "$cdn_count" -ne "$sri_count" ]; then
    echo "❌ FAIL: Not all CDN dependencies have SRI hashes"
    echo "   Found $cdn_count CDN elements, but only $sri_count have integrity"
    failed=1
  fi

  # Check 5: Valid SRI format (sha384-<base64>)
  if echo "$collapsed" | grep -oE '<(script|link)[^>]+>' | grep -qE 'integrity='; then
    if ! echo "$collapsed" | grep -oE '<(script|link)[^>]+>' | \
         grep -E 'integrity=' | grep -q 'sha384-'; then
      echo "❌ FAIL: Invalid SRI format (must be sha384-...)"
      failed=1
    fi
  fi

  if [ $failed -eq 0 ]; then
    echo "✅ PASS"
    echo "   • No unauthorized CDNs"
    echo "   • All versions pinned (no @latest)"
    echo "   • Vue.js uses semver (@X.Y.Z)"
    echo "   • All CDN deps have SRI hashes"
  fi
done

echo ""
if [ $failed -eq 1 ]; then
  echo "=========================================="
  echo "❌ CHECK FAILED"
  echo "=========================================="
  echo ""
  echo "How to fix:"
  echo "  1. Verify CDN hosts are whitelisted: cdn.jsdelivr.net, unpkg.com"
  echo "  2. Remove @latest - pin to exact versions"
  echo "  3. For Vue.js, use semver: vue@2.7.14 (not vue@2)"
  echo "  4. Generate SRI hashes:"
  echo "     curl -s <URL> | openssl dgst -sha384 -binary | openssl base64 -A"
  echo ""
  exit 1
else
  echo "=========================================="
  echo "✅ CHECK PASSED"
  echo "=========================================="
fi
