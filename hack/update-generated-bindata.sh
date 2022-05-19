#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

if [[ ! $(which go-bindata) ]]; then
  echo "go-bindata not found on PATH. To install:"
  echo "go get -u github.com/go-bindata/go-bindata/..."
  exit 1
fi

set -x
go-bindata \
    -nocompress \
    -nometadata \
    -pkg "assets" \
    -prefix "${SCRIPT_ROOT}" \
    -o "${SCRIPT_ROOT}/pkg/assets/bindata.go" \
    ${SCRIPT_ROOT}/manifests/...
