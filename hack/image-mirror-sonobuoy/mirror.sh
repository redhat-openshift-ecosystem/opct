#!/usr/bin/env bash

# Mirror sonobuoy image to OPCT repo.

set -o pipefail
set -o nounset
set -o errexit

export SONOBUOY_VERSION=${SONOBUOY_VERSION:-v0.57.3}
export SONOBUOY_REPO=docker.io/sonobuoy/sonobuoy
export MIRROR_REPO=${MIRROR_REPO:-quay.io/opct/sonobuoy}
export PLATFORM_IMAGES=""

declare -A BUILD_PLATFORMS=()
BUILD_PLATFORMS+=( ["linux-amd64"]="linux/amd64" )
BUILD_PLATFORMS+=( ["linux-arm64"]="linux/arm64" )
BUILD_PLATFORMS+=( ["linux-ppc64le"]="linux/ppc64le" )
BUILD_PLATFORMS+=( ["linux-s390x"]="linux/s390x" )

mkdir -p build/
envsubst < "$(dirname "$0")"/Containerfile > build/sonobuoy.Containerfile

for arch in ${!BUILD_PLATFORMS[*]}
do
    img_name="${MIRROR_REPO}:${SONOBUOY_VERSION}-${arch}"
    podman build --platform "${BUILD_PLATFORMS[$arch]}" \
        -f build/sonobuoy.Containerfile \
        -t "${img_name}" . &&\
        podman push "${img_name}" &&\
        PLATFORM_IMAGES+=" ${img_name}"
done

podman manifest create ${MIRROR_REPO}:${SONOBUOY_VERSION} ${PLATFORM_IMAGES}
podman manifest push ${MIRROR_REPO}:${SONOBUOY_VERSION} docker://${MIRROR_REPO}:${SONOBUOY_VERSION}

