#!/usr/bin/env bash
set -e
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

echo "Testing cluster [$1] [$2] [$3] [$4]"
ocp_version=$1
ocpt_version=$2
provider=$3
profile=$4

# prepare cluster
timestamp=$(date +'%H%M%S')
username=$USER
cluster_name="$username$timestamp"
echo "---- cluster name: $cluster_name"

cluster_home="/tmp/clusters"
cluster_dir="$cluster_home/$cluster_name"
mkdir -p "$cluster_dir"
echo "Cluster dir: $cluster_dir"

# render template
template_dir="$DIR/templates/$provider/$profile"
template_file="$template_dir/install-config.j2.yaml"
data_file="$template_dir/data.yaml"

output_file="$cluster_dir/install-config.yaml"
output_bak="$cluster_dir/install-config.backup.yaml"

cluster_baseDomain="devcluster.openshift.com"
echo "Rendering template: $template_file"
if [ ! -e "$template_file" ]; then
    echo "File $template_file does not exist"
    exit 1
fi

jinja2 "$template_file" "$data_file"\
  -D "cluster_baseDomain=$cluster_baseDomain"\
  -o "$output_file"

cp "$output_file" "$output_bak"

echo "Rendered template: $output_file"

echo "TODO: Create cluster"

echo "TODO: Run compliance tool"

echo "TODO: Collect Reports"

echo "----"