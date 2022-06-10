#!/bin/sh
# Example: ./hack/go-imports.sh pkg/
if [ "$IS_CONTAINER" != "" ]; then
  go install golang.org/x/tools/cmd/goimports@latest
  for TARGET in "${@}"; do
    find "${TARGET}" -name '*.go' ! -name 'bindata.go' ! -path '*/vendor/*' ! -path '*/.build/*' -exec goimports -w {} \+
  done
  git diff --exit-code
else
  podman run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/go/src/github.com/redhat-openshift-ecosystem/provider-certification-tool:z" \
    --workdir /go/src/github.com/redhat-openshift-ecosystem/provider-certification-tool \
    docker.io/golang:1.17 \
    ./hack/go-imports.sh "${@}"
fi
