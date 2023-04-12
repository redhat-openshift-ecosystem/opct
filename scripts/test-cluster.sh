#!/usr/bin/env bash
set -e
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

ocp_version=$1
ocpt_version=$2
profile=$3
echo "- Testing cluster ocp_version[$1] ocpt_version[$2] profile[$3] --"


# prepare cluster
timestamp=$(date +'%H%M%S')
username=$USER
cluster_name="$username$timestamp"
echo "-- Cluster name: $cluster_name"

cluster_home="/tmp/clusters"
cluster_dir="$cluster_home/$cluster_name"
mkdir -p "$cluster_dir"
echo "-- Cluster dir: $cluster_dir"

template_dir="$DIR/templates/$ocp_version/$profile"
if [ ! -d "$template_dir" ]; then
  echo "Template directory does not exist: $template_dir"
  exit 1
fi
echo "-- Template dir: $template_dir"

exit 0
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