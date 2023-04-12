#!/usr/bin/env bash
set -e
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
# Check system
if ! which jinja2 > /dev/null; then
  echo "Error: jinja2 is not available in the system"
  exit 1
fi

# Iterate on profiles
declare -a profiles

profiles=(
    '4.12;0.3;default'
    '4.12;0.2;default'
    '4.12;0.3;compact'
    '4.12;0.3;sno'
    '4.12;0.2;default')

for profile in "${profiles[@]}"
do
    IFS=";" read -r -a arr <<< "${profile}"
    ocp="${arr[0]}"
    opct="${arr[1]}"
    profile="${arr[2]}"
    echo "Delegating  ocp[${ocp}] opct[${opct}] profile[${profile}]"
    "${DIR}"/test-cluster.sh "${ocp}" "${opct}" "${profile}" 
    sleep 3
done