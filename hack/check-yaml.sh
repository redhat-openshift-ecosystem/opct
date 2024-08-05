#!/usr/bin/env bash

set -e

echo "Checking YAML files for syntax errors";

for yaml in data/templates/plugins/*;
do
    echo "#> Rendering YAML file ${yaml}";
    yq4 ea . "${yaml}" >/dev/null;
done
