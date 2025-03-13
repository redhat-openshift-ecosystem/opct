#!/usr/bin/env bash

#
# Script to generate output of opct CLI.
# The generated files are used by:
# - CI to ensure the documentation is updated
# - DOCs to share examples of OPCT stdout
#

DOC_ASSET_OUTPUT=$PWD/docs/assets/output
OUTPUT_DIR=${OUTPUT_OVERRIDE:-${DOC_ASSET_OUTPUT}}


./build/opct-linux-amd64 run --help > "${OUTPUT_DIR}"/opct-run.txt
./build/opct-linux-amd64 retrieve --help > "${OUTPUT_DIR}"/opct-retrieve.txt
./build/opct-linux-amd64 report --help > "${OUTPUT_DIR}"/opct-report.txt
./build/opct-linux-amd64 results --help > "${OUTPUT_DIR}"/opct-results.txt
./build/opct-linux-amd64 status --help > "${OUTPUT_DIR}"/opct-status.txt
./build/opct-linux-amd64 completion --help > "${OUTPUT_DIR}"/opct-completion.txt
./build/opct-linux-amd64 destroy --help > "${OUTPUT_DIR}"/opct-destroy.txt
./build/opct-linux-amd64 version --help > "${OUTPUT_DIR}"/opct-version.txt
./build/opct-linux-amd64 get --help > "${OUTPUT_DIR}"/opct-get.txt
./build/opct-linux-amd64 get images --help > "${OUTPUT_DIR}"/opct-get-images.txt
./build/opct-linux-amd64 sonobuoy --help > "${OUTPUT_DIR}"/opct-sonobuoy.txt
./build/opct-linux-amd64 adm --help > "${OUTPUT_DIR}"/opct-adm.txt
./build/opct-linux-amd64 adm baseline --help > "${OUTPUT_DIR}"/opct-adm-baseline.txt
./build/opct-linux-amd64 adm cleaner --help > "${OUTPUT_DIR}"/opct-adm-cleaner.txt
./build/opct-linux-amd64 adm generate --help > "${OUTPUT_DIR}"/opct-adm-generate.txt
./build/opct-linux-amd64 adm generate checks-docs --help > "${OUTPUT_DIR}"/opct-adm-generate-checks-docs.txt
./build/opct-linux-amd64 adm parse-etcd-logs --help > "${OUTPUT_DIR}"/opct-adm-parse-etcd-logs.txt
./build/opct-linux-amd64 adm parse-metrics --help > "${OUTPUT_DIR}"/opct-adm-parse-metrics.txt
./build/opct-linux-amd64 adm parse-junit --help > "${OUTPUT_DIR}"/opct-adm-parse-junit.txt
./build/opct-linux-amd64 adm e2e-dedicated taint-node --help > "${OUTPUT_DIR}"/opct-adm-e2e-dedicated-taint-node.txt


# TODO(mtulio): move to a map variable and create a function to generate the outputs,
# so we can create a CI check if CLI output has been changed, it requires to update
# static files (which is included in the documentation/markdown files)

# declare -A OPCT_COMMANDS
# OPCT_COMMANDS["run"]="run"
# OPCT_COMMANDS["retrieve"]="retrieve"
# OPCT_COMMANDS["report"]="report"
# OPCT_COMMANDS["results"]="results"
# OPCT_COMMANDS["adm baseline"]="adm-baseline"

# function generate_helper_files() {
#     # root helper
#     ./build/opct-linux-amd64 --help > "${OUTPUT_DIR}"/opct.txt

#     # helper by command
#     for i in "${!OPCT_COMMANDS[@]}"
#     do
#         ./build/opct-linux-amd64 ${i} --help > "${OUTPUT_DIR}"/opct-"${OPCT_COMMANDS[$i]}".txt
#     done
# }

# function check_helper_files() {
#     # generate helper in different directory
#     generate_helper_files
#     OUTPUT_DIR=/tmp/output-gen-cmd
#     mkdir -p $OUTPUT_DIR

#     # check diff across all existing helper files
#     # TODO
# }
