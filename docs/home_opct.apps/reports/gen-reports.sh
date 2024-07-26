#!/usr/bin/env bash

#
# This script helps how to generate many reports
# to analyse the data together.
# The outcome is the extracted and processed OPCT data
# and a index file to be explored.
# Run the file server to explore it: $ python3 -m http.server 3333
#

set -euo pipefail

# Examples:
# gen report-sample-4.14.txt
# upload reports-custom
function helper() {
    echo "
Usage: $0 (gen|upload|serve) (report-file.txt|report-dir)
where:
    gen     generates the report based in the report file.
    upload  generates the report and upload to report directory in S3.
    serve   start http file server in the report directory.

Examples:
1) Generate the report file and upload to S3;
    cat << EOF > ./report-4.14.txt
ocp414rc0_AWS_None_202309222127_sonobuoy_47efe9ef-06e4-48f3-a190-4e3523ff1ae0.tar.gz
ocp414rc2_AWS_AWS_202309290029_sonobuoy_b5b9ce69-8a37-4ca5-aa55-1f4acd6df673.tar.gz=ocp414rc2_AWS_AWS_202309290029
ocp414rc2_AWS_None_202309282349
EOF
    $0 gen report-4.14.txt

2) Upload report to S3 (it's required to be a .txt extension)
    $0 upload report-4.14.txt

3) Start a file server in the report directory. To change the port set the SERVE_PORT=3000
    SERVE_PORT=3000 $0 serve \$PWD/report-4.14
"
}

if [[ -z ${1:-} ]]; then
    echo "Action command not found";
    helper
fi
export ACTION=${1:-}; shift

if [[ -z ${1:-} ]]; then
    echo "Action arg not found";
    helper
fi
export ARG1=${1:-}; shift

set -x

export OPCT_BIN=${OPCT_BIN:-$HOME/opct/bin/opct-devel}
export REPORT_PATH=${REPORT_PATH:-${PWD}/$(basename -s .txt "${ARG1}")}

export RESULTS=();

# Upload to S3 report directory generated files (optional)
# https://openshift-provider-certification.s3.us-west-2.amazonaws.com/index.html
export bucket_name=openshift-provider-certification
export bucket_obj_prefix=home_opct.apps/reports

export upload_files=();
upload_files+=( opct-report.html )
upload_files+=( opct-report.json )
upload_files+=( opct-filter.html )
upload_files+=( metrics.html )
upload_files+=( artifacts_must-gather_camgi.html )
upload_files+=( must-gather/event-filter.html )

FORCE_UPLOAD=${FORCE_UPLOAD:-false}

# Example of report file: ./report-sample.txt
function gen_report_from_file() {

    while read -r line;
    do
        # ignore commented lines
        test "$line" = '#'* && continue
        report_file=$(echo "${line}" | awk -F'=' '{print$1}')
        report_alias=$(echo "${line}" | awk -F'=' '{print$2}')

        if [[ ! -f $report_file ]]; then
            echo "ERROR#1: file [$report_file] not found. Fix and try again."
            exit 1
        fi

        report_file_name=$report_file
        # Create a alias to a friendly report
        if [[ -n $report_alias ]] && [[ ! -L $report_alias ]]; then
            ln -sv "$report_file" "$report_alias"
            report_file_name=$report_alias
        fi
    
        if [[ ! -f $report_file_name ]]; then
            echo "ERROR#2: file [$report_file_name] not found. Fix and try again."
            exit 1
        fi

        RESULTS+=( "$report_file_name" )
    done < "$ARG1"

    # Creating HTML report
    mkdir -vp "${REPORT_PATH}"
    cat <<EOF > "${REPORT_PATH}"/index.html
<!DOCTYPE html>
<html lang="en">
EOF

    # generate the report appending the link to the HTML
    for RES in ${RESULTS[*]}; do 
        echo "# Generating report ${RES}";

        if [[ ! -d "${REPORT_PATH}"/"${RES}" ]]; then
            mkdir -pv "${REPORT_PATH}"/"${RES}"
            ${OPCT_BIN} report "$RES" --loglevel debug --server-skip --save-to "${REPORT_PATH}"/"$RES";
        fi
        echo "<p><a href=\"./${RES}/opct-report.html\">${RES}</a> (<a href=\"./${RES}/metrics.html\">metrics</a>)</p>" >> "${REPORT_PATH}"/index.html
    done

    cat <<EOF >> "${REPORT_PATH}"/index.html
</body>
</html>
EOF
    cat <<EOF > "${REPORT_PATH}"/redirect.html
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8"/>
    <title>OPCT</title>
    <meta http-equiv="refresh" content="0; URL='https://opct.apps.devcluster.openshift.com/reports/report-az/index.html'"/>
</head>
<body></body>
</html>
EOF

}

function upload_report() {
    
    if [[ -z "${ARG1}" ]]; then
        echo "ERROR: unable to read report name. Aborting upload";
        exit 1
    fi
    report_local_path=${ARG1}
    report_prefix=${bucket_obj_prefix}/${report_local_path}
    S3_REPORT_URL=s3://"$bucket_name"/"$report_prefix"

    aws s3 cp "${REPORT_PATH}"/index.html "${S3_REPORT_URL}"/index.html

    for RES in ${RESULTS[*]}; do 
        for OBJ in ${upload_files[*]}; do
            obj_path="${REPORT_PATH}"/"${OBJ}"
            echo "Uploading [${REPORT_PATH}/$obj_path] to [$S3_REPORT_URL/$obj_path]";
            if [[ ${FORCE_UPLOAD} == false ]] ; then
                echo "-> WARN: ignoring upload. Check if the results is correct and run again with FORCE_UPLOAD=true"
            else
                aws s3 cp "${REPORT_PATH}"/"${obj_path}" "${S3_REPORT_URL}"/"${obj_path}"
            fi
        done
    done
}

case $ACTION in
    "gen") gen_report_from_file;;
    "upload") gen_report_from_file; upload_report;;
    "serve") cd "$ARG1" && python3 -m http.server "${SERVE_PORT}" ;;
    *) helper ;;
esac
