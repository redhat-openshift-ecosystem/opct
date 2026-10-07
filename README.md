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
curl -fsSL https://redhat-openshift-ecosystem.github.io/opct/install.sh | bash
```

The installer detects your OS/architecture, verifies the published checksum, and
installs `opct` into `$HOME/.local/bin`. To pin a release or change the
destination:

```bash
curl -fsSL https://redhat-openshift-ecosystem.github.io/opct/install.sh \
  | OPCT_VERSION=v0.6.7 INSTALL_DIR="$HOME/bin" bash
```

If the install directory is not already on your `PATH`, the installer prints the
`export PATH=...` line to add to your shell profile.

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
