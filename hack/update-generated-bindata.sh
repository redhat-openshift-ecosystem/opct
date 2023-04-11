#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# generate static files with go-bindata[1]
# [1] https://github.com/go-bindata/go-bindata
# go-bindata requies minimal version of and go 1.19
# To install go-bindata run:
# $ go get -u github.com/go-bindata/go-bindata/...@latest

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

# io/ioutil is deprecated and go-bindata with the latest version
# is not addressing this recommendation. We are patching this file
# for now and plan to rid-off the go-bindata dependency v0.5+
# https://issues.redhat.com/browse/OPCT-199
#
# Temporary instructions to generate the patch and use on CI:
# # make sure the bindata.go is updated
# $ make update
# # patch the file, replacing ioutil.WriteFile to os.WriteFile
# # copy to file and update the dependencies
# $ cp pkg/assets/bindata.go hack/patches/pkg-assets-bindata.go
# $ make update
# # generate the patch
# $ diff -u pkg/assets/bindata.go hack/patches/pkg-assets-bindata.go > hack/patches/pkg-assets-bindata.go.patch
# $ rm hack/patches/pkg-assets-bindata.go
#
# # restore the patch on CI:
patch pkg/assets/bindata.go < hack/patches/pkg-assets-bindata.go.patch