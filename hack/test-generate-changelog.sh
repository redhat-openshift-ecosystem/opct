#!/usr/bin/env bash

#
# Regression test for hack/generate-changelog.sh.
# Builds a synthetic git history whose commit titles previously broke
# changelog generation (OPCT-495: a title referencing two Jira cards made
# grep emit a multi-line value, producing "sed: unterminated `s' command")
# and asserts the changelog is generated with every reference linked.
#
# Usage: ./hack/test-generate-changelog.sh

set -o errexit
set -o nounset
set -o pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

repo_dir="$tmp_dir/opct-test"
mkdir -p "$repo_dir/hack" "$repo_dir/docs"
cp "$script_dir/generate-changelog.sh" "$repo_dir/hack/"

git -C "$repo_dir" init -q
git -C "$repo_dir" config user.email "test@localhost"
git -C "$repo_dir" config user.name "test"

# base commit and first release
git -C "$repo_dir" commit -q --allow-empty -m "initial commit"
git -C "$repo_dir" tag v0.1.0

# OPCT-495 regression: titles referencing two Jira cards and two PRs
git -C "$repo_dir" commit -q --allow-empty \
    -m "OPCT-462: Re-enable SLO OPCT-011 error budget for upgrade workflow (#254)"
git -C "$repo_dir" commit -q --allow-empty \
    -m "OPCT-999: fix OPCT-998, SPLAT-997 and OCPBUGS-996 (#1) (#2)"
# title with characters that used to be unsafe in sed expressions
git -C "$repo_dir" commit -q --allow-empty \
    -m 'weird title with /slashes/ and $dollar and "quotes"'
# overlapping reference numbers (OPCT-1 vs OPCT-10, #1 vs #10)
# and repeated references in the same title
git -C "$repo_dir" commit -q --allow-empty \
    -m "OPCT-1: touch OPCT-10 and repeat OPCT-1 again (#1) (#10)"
git -C "$repo_dir" tag v0.2.0

# the generator runs git commands in the current directory, not relative
# to the script -- run it from inside the synthetic repo
cd "$repo_dir"
./hack/generate-changelog.sh > /dev/null

changelog="$repo_dir/docs/CHANGELOG.md"
if [ ! -f "$changelog" ]; then
    echo "FAIL: $changelog not generated"
    exit 1
fi

fail=0
check() {
    if grep -qF "$1" "$changelog"; then
        echo "PASS: $1"
    else
        echo "FAIL: $1 not found in $changelog"
        fail=1
    fi
}

check_not() {
    if grep -qF "$1" "$changelog"; then
        echo "FAIL: $1 unexpectedly found in $changelog"
        fail=1
    else
        echo "PASS: absent $1"
    fi
}

check '[OPCT-462](https://issues.redhat.com/browse/OPCT-462)'
check '[OPCT-011](https://issues.redhat.com/browse/OPCT-011)'
check '[OPCT-999](https://issues.redhat.com/browse/OPCT-999)'
check '[OPCT-998](https://issues.redhat.com/browse/OPCT-998)'
check '[SPLAT-997](https://issues.redhat.com/browse/SPLAT-997)'
check '[OCPBUGS-996](https://issues.redhat.com/browse/OCPBUGS-996)'
check '[#254](https://github.com/redhat-openshift-ecosystem/opct/pull/254)'
check '[#1](https://github.com/redhat-openshift-ecosystem/opct/pull/1)'
check '[#2](https://github.com/redhat-openshift-ecosystem/opct/pull/2)'
check '[OPCT-1](https://issues.redhat.com/browse/OPCT-1)'
check '[OPCT-10](https://issues.redhat.com/browse/OPCT-10)'
check '[#10](https://github.com/redhat-openshift-ecosystem/opct/pull/10)'

# mangled-output guards: prefix substitution (OPCT-1 eating OPCT-10)
# and re-processing of generated links must never appear
check_not 'OPCT-1)0'
check_not 'browse/['

if [ "$fail" -ne 0 ]; then
    echo "Regression test FAILED"
    exit 1
fi
echo "All changelog regression tests passed"
