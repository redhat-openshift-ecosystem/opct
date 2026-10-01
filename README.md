# OpenShift Provider Compatibility Tool (`opct`)

OpenShift Provider Compatibility Tool (OPCT) is used to orchestrate workflows for conformance
test suites on OpenShift/OKD installations on cloud providers or hardware.

## Documentation

- [OPCT Overview](https://redhat-openshift-ecosystem.github.io/opct/)
- [User Guide](https://redhat-openshift-ecosystem.github.io/opct/user/)
- [Development Guide](https://redhat-openshift-ecosystem.github.io/opct/devel/guide)

## Getting started

- Download OPCT

```bash
(
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${OPCT_VERSION:-latest}"
REPO="https://github.com/redhat-openshift-ecosystem/opct"

# Reject empty or path-unsafe version strings.
if ! printf '%s' "${VERSION}" | grep -qE '^[A-Za-z0-9._-]+$'; then
  echo "Error: invalid version '${VERSION}'" >&2
  exit 1
fi

if [[ "${VERSION}" == "latest" ]]; then
  BASE_URL="${REPO}/releases/latest/download"
else
  BASE_URL="${REPO}/releases/download/${VERSION}"
fi

OS="$(uname -s)"
ARCH="$(uname -m)"

case "${OS}" in
  Linux)  OS_TAG="linux" ;;
  Darwin) OS_TAG="darwin" ;;
  *)      echo "Error: unsupported OS '${OS}'" >&2; exit 1 ;;
esac

case "${ARCH}" in
  x86_64|amd64)  ARCH_TAG="amd64" ;;
  arm64|aarch64) ARCH_TAG="arm64" ;;
  *)             echo "Error: unsupported architecture '${ARCH}'" >&2; exit 1 ;;
esac

# Published artifacts: linux-amd64, darwin-amd64, darwin-arm64.
if [[ "${OS_TAG}" == "linux" && "${ARCH_TAG}" != "amd64" ]]; then
  echo "Error: no published binary for linux-${ARCH_TAG}" >&2
  exit 1
fi

BINARY="opct-${OS_TAG}-${ARCH_TAG}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf -- "${TMP_DIR}"' EXIT

echo "Downloading ${BINARY} (${VERSION}) ..."
curl -fSL -o "${TMP_DIR}/${BINARY}" "${BASE_URL}/${BINARY}"
curl -fSL -o "${TMP_DIR}/${BINARY}.sum" "${BASE_URL}/${BINARY}.sum"

# The .sum file contains: <md5-hex>  <absolute-CI-build-path>
# Extract the digest (field 1) and ignore the CI path (field 2).
# Note: MD5 verifies download integrity (corruption), not authenticity.
EXPECTED="$(awk '{print $1}' "${TMP_DIR}/${BINARY}.sum")"
if ! printf '%s' "${EXPECTED}" | grep -qiE '^[0-9a-f]{32}$'; then
  echo "Error: malformed checksum in ${BINARY}.sum: '${EXPECTED}'" >&2
  exit 1
fi

if [[ "${OS_TAG}" == "linux" ]]; then
  command -v md5sum >/dev/null 2>&1 \
    || { echo "Error: md5sum not found" >&2; exit 1; }
  ACTUAL="$(md5sum "${TMP_DIR}/${BINARY}" | awk '{print $1}')"
else
  command -v md5 >/dev/null 2>&1 \
    || { echo "Error: md5 not found" >&2; exit 1; }
  ACTUAL="$(md5 -q "${TMP_DIR}/${BINARY}")"
fi

EXPECTED_LOWER="$(printf '%s' "${EXPECTED}" | tr '[:upper:]' '[:lower:]')"
ACTUAL_LOWER="$(printf '%s' "${ACTUAL}" | tr '[:upper:]' '[:lower:]')"
if [[ "${EXPECTED_LOWER}" != "${ACTUAL_LOWER}" ]]; then
  echo "Error: checksum mismatch for ${BINARY}" >&2
  echo "  expected: ${EXPECTED}" >&2
  echo "  actual:   ${ACTUAL}" >&2
  exit 1
fi

mkdir -p "${INSTALL_DIR}"
if command -v install >/dev/null 2>&1; then
  install -m 0755 "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/opct"
else
  cp "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/opct"
  chmod 0755 "${INSTALL_DIR}/opct"
fi

echo "Installed: ${INSTALL_DIR}/opct"

case ":${PATH}:" in
  *:"${INSTALL_DIR}":*)
    ;;
  *)
    echo "Add the install directory to your PATH:"
    echo "  export PATH=\"${INSTALL_DIR}:\${PATH}\""
    ;;
esac
)
```

- Setup a dedicated node to run the test environment (preferred to prevent disruption)

```bash
opct adm e2e-dedicated taint-node
```

- Run regular conformance tests

```bash
opct run --wait
```

- Check the status (optional when not using `--wait` on `run`)

```bash
opct status --wait
```

- Collcet the results

```bash
opct retrieve
```

- Read the report

```bash
opct report *.tar.gz
```

- Destroy the environment

```bash
opct destroy
```

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct, and the process for submitting pull requests to us.
