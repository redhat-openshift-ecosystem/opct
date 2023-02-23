#!/usr/bin/env bash
set -e
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

declare -a profiles

profiles[0]='4.12;0.3;aws;default'
profiles[1]='4.12;0.2;aws;sno'

for profile in "${profiles[@]}"
do
    IFS=";" read -r -a arr <<< "${profile}"
    ocp="${arr[0]}"
    opct="${arr[1]}"
    provider="${arr[2]}"
    profile="${arr[3]}"
    echo "Delegating  [${ocp}] [${opct}] [${provider}] [${profile}]"
    "${DIR}"/test-cluster.sh "${ocp}" "${opct}" "${provider}" "${profile}" #parallel mode: &
    sleep 2
done