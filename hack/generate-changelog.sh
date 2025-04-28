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

# extract_pr_id extracts the Pull Request ID from commmit name.
function extract_pr_id() {
    line="$1"
    # Extracts PR ID from the commit. The main branch has squashed
    # commits with PR ID in the name with formatt:
    # OPCT-ID: description (#PR_ID)
    pr_id=$(echo "${line}" | grep -Po '\(#\d+' | tr -d '\(' || true)
    if [ -n "${pr_id-}" ] ; then
        echo "$pr_id"
        return
    fi
}

# extract_jira_id attempt to extract the Issue ID (Jira) from commmit name.
function extract_jira_id() {
    line="$1"
    # tries to extract OPCT from name
    jira_card=$(echo "${line}" | grep -Po '(OPCT-\d+)' || true)
    if [ -n "${jira_card-}" ] ; then
        echo "$jira_card"
        return
    fi
    # tries to extract OPCT project
    jira_card=$(echo "${line}" | grep -Po '(SPLAT-\d+)' || true)
    if [ -n "${jira_card-}" ] ; then
        echo "$jira_card"
        return
    fi
    # tries to extract OPCT project
    jira_card=$(echo "${line}" | grep -Po '(OCPBUGS-\d+)' || true)
    if [ -n "${jira_card-}" ] ; then
        echo "$jira_card"
        return
    fi
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
        jira_card=$(extract_jira_id "${line}" || true)
        if [ -n "${jira_card-}" ] ; then
            line=$(echo "$line" | sed "s/$jira_card/\[$jira_card\]\(https\:\/\/issues.redhat.com\/browse\/$jira_card\)/")
        fi

        # lookup for PR number (#{\d+}) in the commit name
	    pr_id=$(extract_pr_id "${line}" || true)
        if [ -n "${pr_id-}" ] ; then
            line=$(echo "$line" | sed "s/$pr_id/\[$pr_id\]\(https\:\/\/github.com\/redhat-openshift-ecosystem\/opct\/pull\/${pr_id#\#}\)/")
        fi
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
    jira_card=$(extract_jira_id "${line}" || true)
    if [ -n "${jira_card-}" ] ; then
        line=$(echo $line | sed "s/$jira_card/\[$jira_card\]\(https\:\/\/issues.redhat.com\/browse\/$jira_card\)/")
    fi
    pr_id=$(extract_pr_id "${line}" || true)
    if [ -n "${pr_id-}" ] ; then
        line=$(echo $line | sed "s/$pr_id/\[$pr_id\]\(https\:\/\/github.com\/redhat-openshift-ecosystem\/opct\/pull\/${pr_id#\#}\)/")
    fi
    echo -e "- $commit_url - ${line}" >> "$ch_file"
done

# Phase III: aggregate all generated markdown files into a single CHANGELOG.md ({chagelog_file})

cat > "$chagelog_file" << EOF

# Changelog

Changelog by release for [CLI (OPCT)][project-cli] project.

[project-cli]: https://github.com/redhat-openshift-ecosystem/opct
[project-plugins]: https://github.com/redhat-openshift-ecosystem/provider-certification-plugins

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
for rev_releases in $(ls -r $chagelog_dir --ignore=devel.md); do
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
