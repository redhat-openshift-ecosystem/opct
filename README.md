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
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"

case "$(uname -m)" in
  x86_64|amd64)
    ARCH="amd64"
    ;;
  arm64|aarch64)
    ARCH="arm64"
    ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

BINARY="opct-${OS}-${ARCH}"

curl -fL \
  -o /usr/local/bin/opct \
  "https://github.com/redhat-openshift-ecosystem/opct/releases/download/latest/${BINARY}"

chmod +x /usr/local/bin/opct
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
