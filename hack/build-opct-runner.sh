
set -ex

if [[ -z "${1:-}" ]]; then
  echo "Please set a version to start the build"
  exit 1
fi
OKD_VERSION=$1
podman build -t quay.io/ocp-cert/opct-runner:$OKD_VERSION -f hack/opct-runner/Containerfile hack/opct-runner/
podman push quay.io/ocp-cert/opct-runner:$OKD_VERSION
