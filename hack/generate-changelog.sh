#!/usr/bin/env bash

#
# This script generates changelog fragments for each release,
# ordered it by newest release and publish it in the docs
# page /CHANGELOG.
# TODO: generate changelog for plugins repo.
#

set -o errexit
set -o nounset
set -o pipefail

arg_cmd=${1-}
if [[ -n "$arg_cmd" && "$arg_cmd" != "commits-only" ]]; then
    echo "Error: Invalid argument '$arg_cmd'. Only 'commits-only' is allowed."
    exit 1
fi

releases=($(git tag --list 'v*' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V))

chagelog_file="$(dirname $0)"/../docs/CHANGELOG.md
chagelog_commits_file="$(dirname $0)"/../docs/CHANGELOG_commits.md
chagelog_sum_file="$(dirname $0)"/../docs/CHANGELOG_summary.md

chagelog_dir="/tmp/opct-changelogs"
mkdir -p $chagelog_dir

# extract_jira_ids extracts all Issue IDs (Jira) from commmit name,
# one per line. A title may reference several cards
# (e.g. "OPCT-462: Re-enable SLO OPCT-011 error budget").
function extract_jira_ids() {
    echo "$1" | grep -oE '(OPCT|SPLAT|OCPBUGS)-[0-9]+' || true
}

# extract_pr_ids extracts all Pull Request IDs from commmit name,
# one per line, with the leading '#' (e.g. '#254').
function extract_pr_ids() {
    echo "$1" | grep -oE '\(#[0-9]+' | tr -d '(' || true
}

# link_refs converts all Jira cards and PR references in a commit
# title into markdown links, using bash parameter expansion.
# Never build sed expressions from interpolated variables: a title with
# two cards (OPCT-495) made grep emit a multi-line value that broke the
# sed expression ("unterminated `s' command").
function link_refs() {
    line="$1"
    for jira_card in $(extract_jira_ids "${line}"); do
        line="${line//"$jira_card"/[$jira_card](https:\/\/issues.redhat.com\/browse\/$jira_card)}"
    done
    for pr_id in $(extract_pr_ids "${line}"); do
        pr_num=${pr_id#\#}
        line="${line//"$pr_id"/[$pr_id](https:\/\/github.com\/redhat-openshift-ecosystem\/opct\/pull\/$pr_num)}"
    done
    echo "$line"
}

# Phase 0: prepare the environment
## Clone plugins repository

## Phase I: Create changelog fragment files under {changelog_dir}/{release}.md,

first_commit=$(git rev-list --max-parents=0 HEAD)
init_release=$first_commit
for rel in "${releases[@]}"; do
    echo "Processing release $rel"
    if [[ "$rel" == *","* ]]; then
        echo "Skipping release $rel due to invalid characters (commas)."
        continue
    fi

    ch_file=$chagelog_dir/$rel.md
    echo -e "\n### OPCT [$rel](https://github.com/redhat-openshift-ecosystem/opct/releases/tag/$rel)\n" > "$ch_file"

    # read the git log with changes between releases (from/to)
    git log --pretty=oneline --abbrev-commit --no-decorate --no-color "$init_release"..tags/"$rel" | \
    while read -r line
    do
        commit="$(echo "$line" | awk '{print$1}')"
        commit_url="[$commit](https://github.com/redhat-openshift-ecosystem/opct/commit/$commit)"
        line="${line#"$commit"}"
        line=$(link_refs "${line}")
        echo "- $commit_url - ${line}" >> "$ch_file"
    done
    init_release=$rel
    echo -e "\n\n" >> "$ch_file"
done

## Phase II: create devel.md markdown file with the changes since the last release.

# devel (since last release - need to run from 'main' branch)
ch_file=$chagelog_dir/devel.md
echo -e "\n### OPCT Development\n" > "$ch_file"

# Process OPCT repo
git log --pretty=oneline --abbrev-commit --no-decorate --no-color "$init_release"..HEAD | \
while read -r line
do
    commit="$(echo $line | awk '{print$1}')"
    commit_url="[$commit](https://github.com/redhat-openshift-ecosystem/opct/commit/$commit)"
    line="${line#"$commit"}"
    line=$(link_refs "${line}")
    echo -e "- $commit_url - ${line}" >> "$ch_file"
done

# Phase III: aggregate all generated markdown files into a single CHANGELOG.md ({chagelog_file})

cat > "$chagelog_file" << EOF

# Changelog

Changelog by release for [CLI (OPCT)][project-cli] project.

[project-cli]: https://github.com/redhat-openshift-ecosystem/opct
[project-plugins]: https://github.com/redhat-openshift-ecosystem/opct-plugins

EOF

# if summary file exists, insert it
if [[ -f ${chagelog_sum_file} ]];
then
    cat ${chagelog_sum_file} >> "$chagelog_file"
fi

echo -e "\n## Releases Changes" > "$chagelog_commits_file"

# devel.md will be first
cat $chagelog_dir/devel.md >> "$chagelog_commits_file"

# then append the releases by reverse order (newest version/file first)
for rev_releases in $(ls -r $chagelog_dir | grep -v '^devel\.md$'); do
    echo -e "\n" >> "$chagelog_commits_file"
    cat $chagelog_dir/"$rev_releases" >> "$chagelog_commits_file"
done

# Saving the commits file
cat $chagelog_commits_file >> $chagelog_file

echo -e "\n\n > This page is generated automatically by CI/hack-generate-changelog.sh\n\n" >> "$chagelog_file"

# TODO: create plugin changelog
#plugin_releases=("v0.1.1" "v0.2.2" "v0.3.0" "v0.4.0")

echo "Removing changelog dir ${chagelog_dir}..."
if [[ -n "${chagelog_dir}" && -d "${chagelog_dir}" ]]; then
    rm -rvf "${chagelog_dir}"
fi
