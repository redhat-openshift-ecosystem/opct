# Getting Started

Follow the steps below to get started using the OPCT CLI to schedule a conformance workflow.

## Prerequisites

- An OpenShift/OKD cluster installed
- `KUBECONFIG` environment variable set with cluster admin permissions

## Install <a name="install"></a>

Install the OPCT CLI using the following command:
```sh
# Download the latest release
wget -O ~/bin/opct \
    https://github.com/redhat-openshift-ecosystem/opct/releases/latest/download/opct-linux-amd64

# Make it executable
chmod u+x ~/bin/opct
```

!!! info "See Also"
    - Use the [latest release](https://github.com/redhat-openshift-ecosystem/opct/releases/latest)

## Setup <a name="setup"></a>

Select a [dedicated compute node](./opct/adm/setup-node.md) to be used to host the test environment:
```sh
opct adm setup-node
```

!!! info "See Also"
    - `opct adm setup-node`
    - Reference Diagram
    - Cluster Validation User Guide

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
Explore the results by navigating to the [Web UI](./opct/report.md) to review the [checks](./review/rules), recommendations, and e2e logs.

## Destroy <a name="destroy"></a>
[Destroy](./opct/destroy.md) the test environment:
```sh
opct destroy
```

That's it! You have successfully scheduled the conformance test workflow on an OpenShift cluster using the OPCT CLI, collected the results, and gathered performance data from the cluster.
