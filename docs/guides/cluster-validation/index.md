# Cluster Validation User Guide

Welcome to the user cluster validation user guide.

This page describes the end-to-end steps to validate
an OpenShift/OKD installation on an infrastructure or cloud provider
in conformance with standard test suites.

!!! info "Red Hat Partners"
    Use this guide if you are applying to a Red Hat OpenShift validation program.

    Users not applying to a Red Hat program can explore the [Getting Started guide][getting-started].

[getting-started]: /opct/getting-started


Table Of Contents:

- [Process Overview](#process)
- [Prerequisites](#prerequisites)
    - [Standard Environment](#standard-env)
        - [Setup Dedicated Node](#standard-env-setup-node)
        - [Setup MachineConfigPool (upgrade mode)](#standard-env-setup-mcp)
        - [Testing in a Disconnected Environment](#disconnected-env-setup)
    - [Privilege Requirements](#priv-requirements)
- [Install](#install)
    - [Prebuilt Binary](#install-bin)
    - [Build from Source](#install-source)
- [Usage](#usage)
    - [Run tool](#usage-run)
        - [Default Run mode](#usage-run-regular)
        - [Run 'upgrade' mode](#usage-run-upgrade)
        - [Optional parameters](#usage-run-optional)
    - [Check status](#usage-check)
    - [Collect the results](#usage-retrieve)
    - [Check the Results](#usage-results)
    - [Review the Report](#usage-report)
    - [Submit the Results](#submit-results)
    - [Environment Cleanup](#usage-destroy)
- [Troubleshooting](#troubleshooting)
- [Feedback](#feedback)

## Process Overview <a name="process"></a>

This section outlines the steps for partners to submit conformance results to the Red Hat Connect Portal.

If you do not plan to share the results with Red Hat, you can skip this section.

The following diagram provides an overview of the process:

```mermaid
%%{init: {"flowchart": {"useMaxWidth": false}}}%%

flowchart TD
    subgraph "(A) Partner + Red Hat PTAM"
        A1[Create User Documentation to Install OpenShift in the provider infrastructure]
        A2[Install OpenShift OCP based on the User Documentation]
        A3[Run Conformance Tests]
        A4["Review Conformance Tests (until no checks are failing)"]
        A5[Create Conformance Archive for VCSP - executed on a fresh OCP installation]
        A4 -->|Continuous Improvement| A1
        A1 --> A2 --> A3 --> A4 --> A5
    end

    subgraph "(B)  Partner + Red Hat Portal"
        B1[Open a Ticket in the Partner Portal]
        B2[Upload Reviewed User Documentation and Conformance Results Archive]
        B3[Red Hat Team Reviews the Results]
        B4[Partner Upgrades the Cluster to next minor version]
        B3 -->|Feedback| B2
        A5 --> B1 --> B2 --> B3 --> B4
    end

    subgraph "(C) Finish Process"
        C1[Red Hat Teams Review Final Results and Wrap Up Process]
        B4 --> C1
    end
```

Summary:

- (A): The partner installs an OpenShift cluster using provider-specific features and documents
  the process in a user-guide format based on the official OpenShift documentation. Once the
  documentation is complete and conformance tests pass, the partner must execute conformance
  tests on a fresh installation to ensure no false positives in the final archive.
  For assistance, contact the Red Hat Partner Ecosystem team.
  The acceptance criteria require that no checks fail in the `opct report` command.
- (B): The partner submits a ticket in the Red Hat Connect Portal, attaching the user
  documentation and conformance results. The Red Hat Certification team reviews the submission
  and provides feedback. Once the results are approved, the partner must submit an automated
  upgrade targeting the next minor OpenShift release.
- (C): Red Hat completes the process by reviewing the final results and closing the certification process.

## Prerequisites <a name="prerequisites"></a>

A Red Hat OpenShift 4 cluster must be
[installed](https://docs.openshift.com/container-platform/latest/installing/index.html)
before validation can begin. The OpenShift cluster must be installed
on your infrastructure as if it were a production environment.
Ensure that each feature of your infrastructure that you plan to
support with OpenShift is configured in the cluster (e.g. Load Balancers, Storage, special hardware).

OpenShift supports the following topologies:

- Highly available OpenShift Container Platform cluster (**HA**): Three control plane nodes with any number of compute nodes.
- A three-node OpenShift Container Platform cluster (**Compact**): A compact cluster that has three control plane nodes that are also compute nodes.
- A single-node OpenShift Container Platform cluster (**SNO**): A node that is both a control plane and compute.

OPCT is tested in the following topologies. Any topology flagged as TBD is not currently supported by the tool in the validation process:

| OCP Topology/ARCH | OPCT Initial version | OPCT Execution mode                        |
| --                | --                   | --                                        |
| HA/amd64          | v0.1                 | regular(v0.1+), upgrade(v0.3+), disconnect(v0.4+) |
| HA/arm64          | v0.5                 | all                                       |

!!! info "Unsupported Topologies"
    You can run the tool in unsupported topologies if the required configuration
    is set. However, the report provided by the tool may not be calibrated
    or have the expected results for a formal validation process when applying
    to Red Hat OpenShift programs for Partners.

OpenShift Platform Type supported by OPCT on Red Hat OpenShift validation program:

| Platform Type | Installation method   | Documentation |
| --            | --                    | --            |
| `External`    | `openshift-install`   | [OpenShift Product][ocp-agn] [Providers][ocp-prov] |
| `None`*        | Assisted Installer: `User-managed` network mode | [OpenShift Product][ai-none] |
| `External`    | Agent Based Installer | [OpenShift Product][abi-external] |

*platform type `None` should be used only when required to install OpenShift cluster with Assisted Installer  using `User-Managed` networking mode, otherwise use options with platform type `External`.

[ocp-agn]: https://docs.openshift.com/container-platform/latest/installing/installing_platform_agnostic/installing-platform-agnostic.html
[ocp-prov]: https://docs.providers.openshift.org/platform-external/installing/
[ai-none]: https://docs.openshift.com/container-platform/latest/installing/installing_on_prem_assisted/installing-on-prem-assisted.html
[abi-external]: https://docs.openshift.com/container-platform/latest/installing/installing_with_agent_based_installer/preparing-to-install-with-agent-based-installer.html

!!! info "Unsupported Platform Type"
    You can run the tool in other platform types if the required configuration is set.
    However, the reports may not be calibrated to produce the expected results,
    leading to failures in platform-specific e2e tests requiring special configuration
    or credentials.

The matrix below describes the OpenShift and OPCT versions supported:

| OPCT [version](releases) | OCP tested versions | OPCT Execution mode                |
| --                       | --                  | --                                |
| v0.5.x                   | 4.14-4.18           | regular, upgrade, disconnected     |
| v0.4.x                   | 4.10-4.13           | regular, upgrade, disconnected     |
| v0.3.x                   | 4.9-4.12            | regular, upgrade                   |
| v0.2.x                   | 4.9-4.11            | regular                            |
| v0.1.x                   | 4.9-4.11            | regular                            |

It is highly recommended to use the latest OPCT version.

[releases]: https://github.com/redhat-openshift-ecosystem/opct/releases

### Standard Environment <a name="standard-env"></a>

A dedicated compute node should be used to avoid disruption of the
test scheduler. Otherwise, concurrent workloads, e2e-test manager
(openshift-tests-plugin), and other cluster components (e.g., monitoring)
could disrupt the test environment, leading to unexpected
results (such as eviction of plugins or aggregator server pods).

The **minimum** test environment can match the table below.
Note the differences of minimum RAM for the `Dedicated Test` node
is different than regular compute nodes:

| Machine/Role   | Count | CPU | RAM (GB) | Storage (GB) |
| -------------- | ----- | --- | -------- | ------------ |
| Bootstrap      | 1     | 4   | 16       | 100          |
| Control Plane  | 3     | 4   | 16       | 100          |
| Compute        | 3     | 4   | 16       | 100          |
| Dedicated Test | 1     | 4   | 8        | 100          |

*Note: These requirements are higher than the [minimum installation requirements](https://docs.openshift.com/container-platform/latest/installing/installing_bare_metal/installing-bare-metal.html#installation-minimum-resource-requirements_installing-bare-metal) because of the resource demands of the conformance environment.*

#### Environment Setup: Dedicated Node <a name="standard-env-setup-node"></a>

The `Dedicated Node` is a regular worker with additional labels and taints to run the OPCT environment:

1. Choose one node with at least 8GiB of RAM and 4 CPU.
2. Label the node with `node-role.kubernetes.io/tests=""`.
3. Taint the node with `node-role.kubernetes.io/tests="":NoSchedule`.

Example:

```sh
oc label node <node_name> node-role.kubernetes.io/tests=""
oc adm taint node <node_name> node-role.kubernetes.io/tests="":NoSchedule
```

Starting on v0.5.z you can use a command to find the best node and apply the required taint (deprecated in v0.6+):

```sh
opct adm setup-node
```

Starting on v0.6+, the new command path:

```sh
opct adm e2e-dedicated taint-node
```

#### Setup MachineConfigPool for upgrade tests <a name="standard-env-setup-mcp"></a>

**Note**: A custom `MachineConfigPool` is required only when the OPCT is run in `upgrade` mode. If you are not running upgrade tests, skip this section.

Create a `MachineConfigPool` named `opct` that selects the dedicated node and remains `paused` so that the node won't be rebooted during cluster upgrades:

```bash
cat << EOF | oc create -f -
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfigPool
metadata:
  name: opct
spec:
  machineConfigSelector:
    matchExpressions:
    - key: machineconfiguration.openshift.io/role
      operator: In
      values: [worker,opct]
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/tests: ""
  paused: true
EOF
```

Verify the `MachineConfigPool`:

```bash
oc get machineconfigpool opct
```

#### Testing in a Disconnected Environment <a name="disconnected-env-setup"></a>

OPCT uses several container images during the setup and execution of tests.
See [User Installation Guide - Disconnected Installations][installation-disconnected.md] for details on configuring a mirror registry and directing OPCT to pull images from that mirror.

### Privilege Requirements <a name="priv-requirements"></a>

A user with [cluster admin privileges](https://docs.openshift.com/container-platform/latest/authentication/using-rbac.html#creating-cluster-admin_using-rbac) is required to run the tool. You can also use the default `kubeadmin` user.

## Install <a name="install"></a>

You can download the OPCT binary from [the Project Releases page](https://github.com/redhat-openshift-ecosystem/opct/releases).
Choose the architecture matching the node on which you plan to run the tool:

```sh
BINARY=opct-linux-amd64
wget -O opct --max-redirect=2 "https://github.com/redhat-openshift-ecosystem/opct/releases/download/latest/${BINARY}"
chmod u+x ./opct
```

## Usage <a name="usage"></a>

### Run conformance tests <a name="usage-run"></a>

**Requirements:**
- A dedicated node
- OPCT installed locally

#### Run the default execution mode <a name="usage-run-regular"></a>

```sh
./opct run
```

To watch execution progress:

```sh
./opct run --watch
```

#### Run the `upgrade` mode <a name="usage-run-upgrade"></a>

`upgrade` mode upgrades the cluster to a specified 4.Y+1 release, then runs conformance suites to validate the upgraded cluster:

```sh
./opct run --mode=upgrade --upgrade-to-image=$(oc adm release info 4.Y+1.Z -o jsonpath={.image})
```

**Note**: Before running upgrade mode, you must have created the `MachineConfigPool` named `opct` and installed the `oc` client.

#### Run with Disconnected Mirror registry<a name="usage-run-disconnected"></a>

If you have a disconnected mirror registry configured, run:

```sh
./opct run --image-repository ${TARGET_REPO}
```

### Check status <a name="usage-check"></a>

```sh
./opct status

# Or watch until completion:
./opct status -w
```

### Collect the results <a name="usage-retrieve"></a>

```sh
./opct retrieve
```

Optionally specify a directory:

```sh
./opct retrieve ./destination-dir/
```

### Check the results archive <a name="usage-results"></a>

```sh
./opct results <retrieved-archive>.tar.gz
```

#### Review the report <a name="usage-report"></a>

```sh
./opct report <retrieved-archive>.tar.gz
```

### Submit the results archive <a name="submit-results"></a>

How to submit OPCT results from the validated environment:

- Log in to the [Red Hat Connect Portal](https://connect.redhat.com/login).
- Go to [`Support > My support tickets > Create Case`](https://connect.redhat.com/support/technology-partner/#/case/new).
- In the `Request Category` step, select `Product Certification`.
- In the `Product Selection` step, for the Product field, select `OpenShift Container Platform` and select the version you are using.
- Click `Next` to continue.
- In the `Request Details` step, `Request Summary` field, specify `[VCSP] OPCT Test Results <provider name>` and provide any additional details in the `Please add description` box.
- Click `Next` to continue.
- Click `Submit` when you have completed all the required information.
- A Product Certification ticket will be created. Please follow the instructions provided to add the test results and any other related material for review.
- Go to [`Support > My support tickets`](https://connect.redhat.com/support/technology-partner/#/case/list) to find the case and review status or to add comments.

Required files to attach to a new support case:

- Attach the detailed Deployment Document describing how the cluster is
  installed, including architecture, flavors, and additional/specific
  configurations from your validated Cloud Provider.
- Download, review and attach the [`user-installation-checklist.md`][installation-checklist] to the case.
- Attach the `<retrieved-archive>.tar.gz` result file to the case.


### Environment Cleanup <a name="usage-destroy"></a>

When validation is complete, destroy the conformance environment:

```sh
./opct destroy
```

You must manually remove the OpenShift cluster afterward.

## Troubleshooting Helper

For issues or investigating test failures, see:

- [Troubleshooting Guide][troubleshooting-guide]
- [Installation Review][installation-review]

## Feedback <a name="feedback"></a>

If you are a community user and encounter bugs, open a [new GitHub issue][opct-new-issue].

If you are undergoing a partner validation process, contact your Red Hat Partner Manager for official conformance assessment support.

[installation-disconnected.md]: ./installation-disconnected.md
[installation-checklist]: ./installation-checklist.md
[installation-review]: ./installation-review.md
[troubleshooting-guide]: ./../../review/troubleshooting.md
[opct-new-issue]: https://github.com/redhat-openshift-ecosystem/opct/issues/new
