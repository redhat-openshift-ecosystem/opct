#!/bin/sh

if [ "$IS_CONTAINER" != "" ]; then
  go install github.com/go-bindata/go-bindata/go-bindata@latest
  set -xe
  ./hack/update-generated-bindata.sh
  set +ex
  git diff --exit-code
else
  podman run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/go/src/github.com/redhat-openshift-ecosystem/provider-certification-tool:z" \
    --workdir /go/src/github.com/redhat-openshift-ecosystem/provider-certification-tool \
    docker.io/golang:1.19 \
    ./hack/verify-codegen.sh "${@}"
fi
