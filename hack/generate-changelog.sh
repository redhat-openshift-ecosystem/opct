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

releases=("v0.1.0" "v0.2.0" "v0.3.0" "v0.4.0")

chagelog_file="$(dirname $0)"/../docs/CHANGELOG.md
chagelog_dir="$(dirname $0)"/../docs/changelogs
chagelog_dir="/tmp/opct-changelogs"
mkdir -p $chagelog_dir

cat > "$chagelog_file" << EOF

# CHANGELOG

EOF

first_commit=$(git rev-list --max-parents=0 HEAD)
init_release=$first_commit
for rel in ${releases[*]}; do
    ch_file=$chagelog_dir/$rel.md
    echo -e "\n# [$rel](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/releases/tag/$rel)" > "$ch_file"
    echo -e "OPCT:\n" >> "$ch_file"
    git log --pretty=oneline --abbrev-commit --no-decorate --no-color $init_release..tags/$rel | \
    while read line
    do
        commit="$(echo $line | awk '{print$1}')"
        commit_url="[$commit](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/commit/$commit)"
        line="${line#"$commit"}"
        jira_card=$(echo $line | grep -Po '(OPCT-\d+)' || true)
        if [ -n "${jira_card-}" ] ; then
            line=$(echo $line | sed "s/$jira_card/\[$jira_card\]\(https\:\/\/issues.redhat.com\/browse\/$jira_card\)/")
        fi
        pr_id=$(echo $line | grep -Po '#\d+' || true)
        if [ -n "${pr_id-}" ] ; then
            line=$(echo $line | sed "s/$pr_id/\[$pr_id\]\(https\:\/\/github.com\/redhat-openshift-ecosystem\/provider-certification-tool\/pull\/${pr_id#\#}\)/")
        fi
        echo -e "- $commit_url - ${line}" >> "$ch_file"
    done
    init_release=$rel
    echo -e "\n\n" >> "$ch_file"
done

# devel (since last release - need to run from 'main' branch)
ch_file=$chagelog_dir/devel.md
echo -e "\n# Development\n" > "$ch_file"
echo -e "OPCT:\n" >> "$ch_file"
git log --pretty=oneline --abbrev-commit --no-decorate --no-color $init_release..HEAD | \
while read line
do
    commit="$(echo $line | awk '{print$1}')"
    commit_url="[$commit](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/commit/$commit)"
    line="${line#"$commit"}"
    jira_card=$(echo $line | grep -Po '(OPCT-\d+)' || true)
    if [ -n "${jira_card-}" ] ; then
        line=$(echo $line | sed "s/$jira_card/\[$jira_card\]\(https\:\/\/issues.redhat.com\/browse\/$jira_card\)/")
    fi
    pr_id=$(echo $line | grep -Po '#\d+' || true)
    if [ -n "${pr_id-}" ] ; then
        line=$(echo $line | sed "s/$pr_id/\[$pr_id\]\(https\:\/\/github.com\/redhat-openshift-ecosystem\/provider-certification-tool\/pull\/${pr_id#\#}\)/")
    fi
    echo -e "- $commit_url - ${line}" >> "$ch_file"
done

cat > "$chagelog_file" << EOF

# CHANGELOG

EOF

cat $chagelog_dir/devel.md >> "$chagelog_file"
for rev_releases in $(ls -r $chagelog_dir --ignore=devel.md); do
    echo -e "\n" >> "$chagelog_file"
    cat $chagelog_dir/$rev_releases >> "$chagelog_file"
done

echo -e "\n\n > This page is generated automatically by CI/hack-generate-changelog.sh\n\n" >> "$chagelog_file"


# TODO: create plugin change log
#plugin_releases=("v0.1.1" "v0.2.2" "v0.3.0" "v0.4.0")