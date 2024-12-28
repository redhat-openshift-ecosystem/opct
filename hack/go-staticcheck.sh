#!/bin/sh
# Example:  ./hack/go-staticcheck.sh ./...

if [ "$IS_CONTAINER" != "" ]; then
    go install honnef.co/go/tools/cmd/staticcheck@latest
    staticcheck "${@}"
else
  podman run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/go/src/github.com/redhat-openshift-ecosystem/opct:z" \
    --workdir /go/src/github.com/redhat-openshift-ecosystem/opct \
    docker.io/golang:1.19 \
    ./hack/go-staticcheck.sh "${@}"
fi
