# OPCT | OpenShift Provider Compatibility Tool

Welcome to the documentation for the OpenShift Provider Compatibility Tool (OPCT)!

The OPCT tool is used to orchestrate conformance test workflows on
OpenShift/OKD clusters, providing a consolidated report to the end-users.

The tool is based on the
<a href="https://sonobuoy.io" target="_blank">Sonobuoy</a>
utility to orchestrate and manage the test environment,
setting up OpenShift conformance workflows as
<a href="https://sonobuoy.io/plugins" target="_blank">Sonobuoy Plugins</a>
using the `openshift-tests` utility, which is responsible for orchestrating the
OpenShift/OKD and Kubernetes conformance test suites.

## Quick Start

To get started, take a look at the [How It Works](#how-it-works) and [Getting Started](#getting-started) sections
if you would like to explore the tool.

If you are a **Red Hat Partner** applying to a Red Hat OpenShift validation
program, start by exploring the [Cluster Validation Guides](./guides/cluster-validation/index.md).

## How It Works

1) <a href="https://docs.openshift.com/container-platform/latest/installing/overview/index.html" target="_blank">Install an OpenShift/OKD cluster</a> in the cloud provider or hardware using any valid installation method.

The following diagram illustrates an example of a typical OpenShift cluster installed on a cloud provider connected to the internet, using a dedicated node to orchestrate the test environment:

![Reference Topology of OpenShift cluster installed on a cloud provider](./diagrams/ocp-ha-opct.diagram.png)

2) [Download OPCT](./getting-started.md#install) and set up the dedicated node:

```sh
./opct adm e2e-dedicated taint-node
```

3) [Schedule](./getting-started.md#run) the conformance jobs:

```sh
./opct run --watch
```

--8<-- "docs/diagrams/opct-sequence-diagram_small_run.md"

4) Explore the [results](./getting-started.md#results):

```sh
./opct retrieve && ./opct report -s ./results opct_*.tar.gz
```

--8<-- "docs/diagrams/opct-sequence-diagram_small_results.md"

## Getting Started

Here you can find the initial steps to use the tool:

!!! info "Red Hat OpenShift VCSP Program"
    If you are  Red Hat partner applying to an OpenShift validation program,
    refers to the [Cluster Validation User Guide][user-guide] to get started.

- [Getting Started][getting-started]
- [`opct` CLI Reference](./opct/index.md)
- [Cluster Validation User Guide][user-guide]
- [Support Guide](./guides/support-guide.md)
- [Development Guide](./devel/guide.md)


[getting-started]: ./getting-started.md
[user-guide]: ./guides/cluster-validation/index.md
[sonobuoy]: https://sonobuoy.io
[sb-plugins]: https://sonobuoy.io/plugins
