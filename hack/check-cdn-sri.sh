#!/bin/bash
# CDN SRI (Subresource Integrity) verification script
#
# Usage:
#   ./hack/check-cdn-sri.sh              # validate production HTML templates
#   ./hack/check-cdn-sri.sh --test-fixtures  # run adversarial regression tests
#
# This script validates that all CDN dependencies in HTML templates:
# 1. Use only whitelisted CDN hosts (cdn.jsdelivr.net, unpkg.com)
# 2. Have exact pinned versions (X.Y.Z semver for every CDN package)
# 3. Include valid SRI SHA-384 hashes on each CDN script/link element

set -e

URL_CHARS='[^"'"'"' >]'
CDN_HOST_PATTERN='^https://(cdn\.jsdelivr\.net|unpkg\.com)$'
SEMVER_PATTERN='^[0-9]+\.[0-9]+\.[0-9]+$'
SRI_PATTERN='^sha384-[A-Za-z0-9+/]{64}$'

validate_file() {
  local file=$1
  local file_failed=0

  echo ""
  echo "Checking $file..."

  if [ ! -f "$file" ]; then
    echo "❌ FAIL: file not found: $file"
    return 1
  fi

  collapsed=$(tr '\n' ' ' < "$file")
  elements=$(echo "$collapsed" | grep -oE '<(script|link)[^>]+>' || true)

  # Check 1: no unauthorized CDN hosts in script/link elements
  bad_cdns=$(echo "$elements" | grep -oE "https://${URL_CHARS}+" | \
    grep -oE 'https://[^/]+' | grep -Ev "$CDN_HOST_PATTERN" || true)
  if [ -n "$bad_cdns" ]; then
    echo "❌ FAIL: Found unauthorized CDN hosts in script/link tags"
    echo "$bad_cdns"
    file_failed=1
  fi

  # Check 2: no @latest versions
  if grep -q '@latest' "$file"; then
    echo "❌ FAIL: Found @latest (unpinned) versions"
    grep -n '@latest' "$file" || true
    file_failed=1
  fi

  # Check 3: every CDN URL must pin an exact X.Y.Z version
  cdn_urls=$(echo "$elements" | grep -oE "https://${URL_CHARS}+" | \
    grep -E 'https://(cdn\.jsdelivr\.net|unpkg\.com)' || true)
  if [ -n "$cdn_urls" ]; then
    while IFS= read -r url; do
      [ -z "$url" ] && continue
      version=$(echo "$url" | grep -oE '@[^/"]+' | head -1 | cut -c2-)
      if [ -z "$version" ]; then
        echo "❌ FAIL: CDN URL missing exact version (expected @X.Y.Z): $url"
        file_failed=1
        continue
      fi
      if ! echo "$version" | grep -Eq "$SEMVER_PATTERN"; then
        echo "❌ FAIL: CDN URL not pinned to exact semver X.Y.Z (got @${version}): $url"
        file_failed=1
      fi
    done <<< "$cdn_urls"
  fi

  # Check 4: each CDN script/link element must include a valid SRI hash
  while IFS= read -r element; do
    [ -z "$element" ] && continue
    echo "$element" | grep -qE 'https://(cdn\.jsdelivr\.net|unpkg\.com)' || continue

    integrity=$(echo "$element" | grep -oE 'integrity=(["'"'"'])([^"'"'"']*)\1' | \
      head -1 | sed -E 's/^integrity=(["'"'"'])(.*)\1$/\2/')
    if [ -z "$integrity" ]; then
      echo "❌ FAIL: CDN element missing integrity attribute"
      echo "   $element"
      file_failed=1
      continue
    fi
    if ! echo "$integrity" | grep -Eq "$SRI_PATTERN"; then
      echo "❌ FAIL: Invalid SRI format (expected sha384-<64-char base64>): $integrity"
      echo "   $element"
      file_failed=1
    fi
  done <<< "$elements"

  if [ "$file_failed" -eq 0 ]; then
    echo "✅ PASS"
    echo "   • No unauthorized CDNs"
    echo "   • All versions pinned to exact X.Y.Z semver"
    echo "   • All CDN elements have valid SRI SHA-384 hashes"
  fi

  return "$file_failed"
}

run_production_checks() {
  local failed=0

  echo "=========================================="
  echo "CDN SECURITY CHECK"
  echo "=========================================="

  for file in data/templates/report/report.html data/templates/report/filter.html; do
    if ! validate_file "$file"; then
      failed=1
    fi
  done

  echo ""
  if [ "$failed" -eq 1 ]; then
    echo "=========================================="
    echo "❌ CHECK FAILED"
    echo "=========================================="
    echo ""
    echo "How to fix:"
    echo "  1. Verify CDN hosts are whitelisted: cdn.jsdelivr.net, unpkg.com"
    echo "  2. Remove @latest and partial versions - pin every package to @X.Y.Z"
    echo "  3. Generate SRI hashes:"
    echo "     curl -s <URL> | openssl dgst -sha384 -binary | openssl base64 -A"
    echo ""
    return 1
  fi

  echo "=========================================="
  echo "✅ CHECK PASSED"
  echo "=========================================="
  return 0
}

run_fixture_tests() {
  local fixture_dir="test/testdata/cdn-sri"
  local failed=0

  echo "=========================================="
  echo "CDN SRI ADVERSARIAL FIXTURE TESTS"
  echo "=========================================="

  for fixture in "$fixture_dir"/bad-*.html; do
    [ -e "$fixture" ] || continue
    echo ""
    echo "Expect FAIL: $fixture"
    if validate_file "$fixture"; then
      echo "❌ REGRESSION: expected validator to reject $fixture"
      failed=1
    else
      echo "✅ Correctly rejected"
    fi
  done

  for fixture in "$fixture_dir"/good-*.html; do
    [ -e "$fixture" ] || continue
    echo ""
    echo "Expect PASS: $fixture"
    if validate_file "$fixture"; then
      echo "✅ Correctly accepted"
    else
      echo "❌ REGRESSION: expected validator to accept $fixture"
      failed=1
    fi
  done

  echo ""
  if [ "$failed" -eq 1 ]; then
    echo "=========================================="
    echo "❌ FIXTURE TESTS FAILED"
    echo "=========================================="
    return 1
  fi

  echo "=========================================="
  echo "✅ FIXTURE TESTS PASSED"
  echo "=========================================="
  return 0
}

case "${1:-}" in
  --test-fixtures)
    run_fixture_tests
    ;;
  "")
    run_production_checks
    run_fixture_tests
    ;;
  *)
    echo "Usage: $0 [--test-fixtures]" >&2
    exit 2
    ;;
esac
