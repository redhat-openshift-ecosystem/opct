# Getting Started

Follow the steps below to get started using the OPCT CLI to schedule a conformance workflow.

## Prerequisites

- An OpenShift/OKD cluster installed
- `KUBECONFIG` environment variable set with cluster admin permissions

## Install <a name="install"></a>

Install the OPCT CLI using the following command:
```sh
curl -fsSL https://redhat-openshift-ecosystem.github.io/opct/install.sh | bash
```

The installer detects your OS/architecture, verifies the published checksum, and
installs `opct` into `$HOME/.local/bin`.

To pin a specific release or change the destination directory:

```sh
curl -fsSL https://redhat-openshift-ecosystem.github.io/opct/install.sh \
  | OPCT_VERSION=v0.6.7 INSTALL_DIR="$HOME/bin" bash
```

If the install directory is not already on your `PATH`, the installer prints the
`export PATH=...` line to add to your shell profile.

!!! info "See Also"
    - Review the [installer source](https://redhat-openshift-ecosystem.github.io/opct/install.sh) before piping it to a shell
    - Use the [latest release](https://github.com/redhat-openshift-ecosystem/opct/releases/latest)

## Setup <a name="setup"></a>

Select a [dedicated compute node](./opct/adm/e2e-dedicated-taint-node.md) to be used to host the test environment:
```sh
opct adm e2e-dedicated taint-node
```

!!! info "See Also"
    - [`opct adm e2e-dedicated taint-node` CLI reference](./opct/adm/e2e-dedicated-taint-node.md)
    - [Reference Diagram](./diagrams/ocp-architecture-reference.md)
    - [Cluster Validation User Guide](./guides/cluster-validation/index.md)

## Run <a name="run"></a>

[Schedule](./opct/run.md) the default workflow and monitor its execution:
```sh
opct run --watch
```

!!! info "See Also"
    - `opct run`
    - Reference Diagram
    - Cluster Validation User Guide

## Results <a name="results"></a>

<a name="retrieve"></a>
Once the workflow completes, [retrieve](./opct/retrieve.md) the results:
```sh
opct retrieve
```

<a name="report"></a>
Generate the [consolidated report](./opct/report.md):
```sh
opct report --save-to ./results opct*.tar.gz
```

<a name="explore"></a>
Explore the results by navigating to the [Web UI](./opct/report.md) to review the [checks](review/rules.md), recommendations, and e2e logs.

## Destroy <a name="destroy"></a>
[Destroy](./opct/destroy.md) the test environment:
```sh
opct destroy
```

That's it! You have successfully scheduled the conformance test workflow on an OpenShift cluster using the OPCT CLI, collected the results, and gathered performance data from the cluster.
