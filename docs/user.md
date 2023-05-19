# User Guide

This document describes the process of testing an OpenShift cluster with the OpenShift Provider Certification Tool (OPCT). It describes in detail the necessary steps to provision and configure the cluster for test execution, as well as executing the OCPT tool.

> Disclaimer: This document is under development and released as a `preview`.

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
    - [Submit the Results](#submit-results)
    - [Environment Cleanup](#usage-destroy)
- [Troubleshooting](#troubleshooting)
- [Feedback](#feedback)

## Process Overview <a name="process"></a>

This section describes the steps of the process when submitting the results to Red Hat Partner.
If the goal is not sharing the results to Red Hat, you can go to the next section.

Overview of the process to apply the results to the Red Hat Partner support case:

1. Define the OpenShift version and cluster topology to be tested
1. Install an OpenShift cluster on **the specific version and topology** to be tested
1. Prepare the OpenShift cluster to run the tests
1. Download and install the OPCT tool
1. Run the OPCT tool
1. Monitor test execution
1. Gather test results
1. Share the test results with Red Hat by opening a support case


Do not expect all tests to pass in the first run. It's not uncommon for some tests to occasionally fail.  As a result, you may be asked to work with Support Engineers to adjust settings and repeat the process a few times, depending on the submitted results.

Additionally, partners may be asked to manually upgrade the cluster to the next minor release and re-submit test results. This is referred to as the "upgrade" OPCT Execution Mode.

More detail on each step can be found in the sections further below.

## Prerequisites <a name="prerequisites"></a>

A Red Hat OpenShift 4 cluster must be [installed](https://docs.openshift.com/container-platform/latest/installing/index.html) before validation can begin. The OpenShift cluster must be installed on your infrastructure as if it were a production environment. Ensure that each feature of your infrastructure you plan to support with OpenShift is configured in the cluster (e.g. Load Balancers, Storage, special hardware).

The table below describes the OpenShift and OPCT versions supported for each release and features:

| OPCT [version](releases) | OCP Supported versions | OPCT Execution mode |
| -- | -- | -- |
| v0.3.x | 4.9, 4.10, 4.11, 4.12 | regular, upgrade |
| v0.2.x | 4.9, 4.10, 4.11 | regular |
| v0.1.x | 4.9, 4.10, 4.11 | regular |

It's highly recommended to use the latest OPCT version.

[releases]:https://github.com/redhat-openshift-ecosystem/provider-certification-tool/releases

### Standard Clusters <a name="standard-env"></a>

A dedicated compute node should be used to execute tests, to avoid interference with the test scheduler. Otherwise, the concurrency between resources scheduled on the cluster, e2e-test manager (aka openshift-tests-plugin), and other stacks like monitoring can interfere with the test environment, leading to unexpected results, like the eviction of plugins or aggregator server (`sonobuoy` pod).

The dedicated node environment cluster size can be adjusted to match the table below. Note the differences in the `Dedicated Test` machine:

Recommended resources:

| Machine       | Count | CPU | RAM (GB) | Storage (GB) |
| ------------- | ----- | --- | -------- | ------------ |
| Bootstrap     | 1     | 4   | 16       | 100          |
| Control Plane | 3     | 4   | 16       | 100          |
| Compute       | 3     | 4   | 16       | 100          |
| Dedicated Test| 1     | 4   | 8        | 100          |

*Note: These requirements are higher than the [minimum requirements](https://docs.openshift.com/container-platform/latest/installing/installing_bare_metal/installing-bare-metal.html#installation-minimum-resource-requirements_installing-bare-metal) for a regular cluster (non-validation) in OpenShift product documentation due to the resource demand of the conformance environment.*

#### Environment Setup: Dedicated Node <a name="standard-env-setup-node"></a>

The `Dedicated Node` is a regular worker with additional label and taints to run the OPCT environment.

The following requirements must be satisfied:

1. Choose one node with at least 8GiB of RAM and 4 vCPU
2. Label node with `node-role.kubernetes.io/tests=""` (conformance-related pods will schedule to dedicated node)
3. Taint node with `node-role.kubernetes.io/tests="":NoSchedule` (prevent other pods from running on dedicated node)

There are two options to accomplish this type of setup:

##### Option A: Command Line

```shell
oc label node $NODE_NAME node-role.kubernetes.io/tests=""
oc adm taint node $NODE_NAME node-role.kubernetes.io/tests="":NoSchedule
```

##### Option B: Machine Set

If you have support for OpenShift's Machine API then you can create a new `MachineSet` to configure the labels and taints. See [OpenShift documentation](https://docs.openshift.com/container-platform/latest/machine_management/creating-infrastructure-machinesets.html#binding-infra-node-workloads-using-taints-tolerations_creating-infrastructure-machinesets) on how to configure a new `MachineSet`. Note that at the time of conformance testing, OpenShift's product documentation may not mention your infrastructure provider yet!

Here is a `MachineSet` YAML snippet on how to configure the label and taint as well:

```yaml
      metadata:
        labels:
          node-role.kubernetes.io/tests: ""
      taints:
        - key: node-role.kubernetes.io/tests
          effect: NoSchedule
```

#### Setup MachineConfigPool for upgrade tests <a name="standard-env-setup-mcp"></a>

**Note**: The `MachineConfigPool` should be created only when the OPCT execution mode (`--mode`) is `upgrade`. If you are not running upgrade tests, please skip this section.

One `MachineConfigPool`(MCP) with the name `opct` must be created, selecting the dedicated node labels. The MCP must be `paused`, thus the node running the validation environment will not be restarted while the cluster is upgrading, avoiding disruptions to the conformance results.

You can create the `MachineConfigPool` by running the following command:

```bash
cat << EOF | oc create -f -
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfigPool
metadata:
  name: opct
spec:
  machineConfigSelector:
    matchExpressions:
    - key: machineconfiguration.openshift.io/role,
      operator: In,
      values: [worker,opct]
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/tests: ""
  paused: true
EOF
```

Make sure the `MachineConfigPool` has been created correctly:

```bash
oc get machineconfigpool opct
```

### "Compact" Clusters
In the "compact" topology we allow both the control plane and compute pods to be scheduled on the same nodes. This topology maintain redundancy for high-availability, while reducing costs. This can be an interesting alternative to edge and other reduced deployments.

Recommended resources:

| Machine           | Count | CPU | RAM (GB) | Storage (GB) |
|-------------------| ----- |-----|----------| ------------ |
| Bootstrap         | 1     | 4   | 16       | 100          |
| Control + Compute | 3     | 8   | 32       | 100          |
| Dedicated Test    | 1     | 4   | 8        | 100          |

To create a "compact" cluster, ensure that you have set the number of worker replicas to zero. Check the "Configuring a three-node cluster" section of the [OpenShift documentation](https://docs.openshift.com/container-platform/4.12/installing/installing_bare_metal/installing-bare-metal.html#installation-three-node-cluster_installing-bare-metal) for more information about configuring a three-node cluster.

Compact clusters can be tested and reported with the same process describe for standard clusters.

### "Single Node" Clusters
In the "single node" topology we execute all pods in a single node without redundancy.
This topology can not only reduce costs, but simplify the provisioning of development environments.

Recommended resources:

| Machine           | Count | CPU | RAM (GB) | Storage (GB) |
|-------------------|-------|-----|----------| ------------ |
| Bootstrap         | 1     | 4   | 16       | 100          |
| Control + Compute | 1     | 32  | 256      | 100          |
| Dedicated Test    | 1     | 4   | 8        | 100          |

To create a "single node" cluster, ensure that you have set the number of worker replicas to zero and the number of masters set to one. Check the "Installing OpenShift on a single node" section of the [OpenShift documentation](https://docs.openshift.com/container-platform/4.12/installing/installing_sno/install-sno-installing-sno.html) for more information about configuring a single node cluster.

Single node clusters can be tested and reported with the same process describe for standard clusters.

#### Testing in a Disconnected Environment <a name="disconnected-env-setup"></a>

The OPCT requires numerous images during the setup and execution of tests.
See [User Installation Guide - Disconnected Installations](./user-installation-disconnected.md) for details
on how to configure a mirror registry and how to run the OPCT to rely on the mirror
registry for images.

### Privilege Requirements <a name="priv-requirements"></a>

A user with [cluster administrator privilege](https://docs.openshift.com/container-platform/latest/authentication/using-rbac.html#creating-cluster-admin_using-rbac) must be used to run the tool. The default `kubeadmin` user has sufficient privileges and can be used to run the tool.

## Install <a name="install"></a>

The OPCT is shipped as a single executable binary which can be downloaded from [the Project Releases page](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/releases). Choose the latest version and the architecture of the node (client) you will execute the tool, then download the binary.

The tool can be used from any system with access to API to the OpenShift cluster under test.

## Usage <a name="usage"></a>

### Run conformance tests <a name="usage-run"></a>

Requirements:

- You have set the dedicated node
- You have installed OPCT

#### Run the default execution mode <a name="usage-run-regular"></a>

- Create and run the validation environment (detaching the terminal/background):

```sh
openshift-provider-cert run 
```

#### Run the `upgrade` mode <a name="usage-run-upgrade"></a>

The `upgrade` mode runs the OpenShift cluster updates to the `4.y+1` version, then the regular conformance suites will be executed (Kubernetes and OpenShift). This mode was created to validate the entire update process, and to make sure the target OCP release is validated on the conformance suites.

> Note: If you will submit the results to Red Hat Partner Support, you must have Validated the installation on the initial release using the regular execution. For example, to submit the upgrade tests for 4.11->4.12, you must have submitted the regular tests for 4.11. If you have any questions, ask your Red Hat Partner Manager.

Requirements for running the `upgrade` mode:

- You have created the `MachineConfigPool` with name `opct`
- You have installed the OpenShift client locally (`oc`) - or have noted the Image `Digest` of the target release
- You must choose the next release of Y-stream (`4.Y+1`) supported by your current release. (See [update graph](https://access.redhat.com/labs/ocpupgradegraph/update_path))

```sh
openshift-provider-cert run --mode=upgrade --upgrade-to-image=$(oc adm release info 4.Y+1.Z -o jsonpath={.image})
```

#### Run with the Disconnected Mirror registry<a name="usage-run-disconnected"></a>

Tests can be executed in a disconnected environment through the use of a mirror registry.

Requirements for running tests with a disconnected mirror registry:

- Disconnected Mirror Image Registry created
- [Private cluster Installed](https://docs.openshift.com/container-platform/latest/installing/installing_bare_metal/installing-restricted-networks-bare-metal.html)
- [You created a registry on your mirror host](https://docs.openshift.com/container-platform/latest/installing/disconnected_install/installing-mirroring-installation-images.html#installing-mirroring-installation-images)


To run tests such that they use images hosted by the Disconnected Mirror registry:

~~~sh
openshift-provider-cert run --image-repository ${TARGET_REPO}
~~~

#### Optional parameters for run <a name="usage-run-optional"></a>

- Create and run the validation environment and keep watching the progress:
```sh
openshift-provider-cert run -w
```

### Check status <a name="usage-check"></a>

```sh
openshift-provider-cert status

# OR Keep watch open until completion

openshift-provider-cert status -w
```

### Collect the results <a name="usage-retrieve"></a>

The results must be retrieved from the OpenShift cluster under test using:

```sh
openshift-provider-cert retrieve

# OR save to the target directory

openshift-provider-cert retrieve ./destination-dir/
```

The file must be saved locally.

### Check the results archive <a name="usage-results"></a>

You can see a summarized view of the results using:

```sh
openshift-provider-cert results <retrieved-archive>.tar.gz
```

### Submit the results archive <a name="submit-results"></a>

How to submit OPCT results from the validated environment:

- Log in to the [Red Hat Connect Portal](https://connect.redhat.com/login).
- Go to [`Support > My support tickets > Create Case`](https://connect.redhat.com/support/technology-partner/#/case/new).
- In the `Request Category` step, select `Product Certification`.
- In the `Product Selection` step, for the Product field, select `OpenShift Container Platform` and select the Version you are using.
- Click `Next` to continue.
- In the `Request Details` step, in the `Request Summary` field, specify `[VCSP] OPCT Test Results <provider name>` and provide any additional details in the `Please add description` field.
- Click `Next` to continue.
- Click `Submit` when you have completed all the required information.
- A Product Certification ticket will be created, and please follow the instructions provided to add the test results and any other related material for us to review.
- Go to [`Support > My support tickets`](https://connect.redhat.com/support/technology-partner/#/case/list) to find the case and review status and/or to add comments to the case.

Required files to attach to a NEW support case:

- Attach the detailed Deployment Document describing how the cluster is installed, architecture, flavors and additional/specific configurations from your validated Cloud Provider.
- Download, review and attach the [`user-installation-checklist.md`](./user-installation-checklist.md) to the case.
- Attach the `<retrieved-archive>.tar.gz` result file to the case.


### Environment Cleanup <a name="usage-destroy"></a>

Once the validation process is complete and you are comfortable with destroying the test environment:

```sh
openshift-provider-cert destroy
```

You will need to destroy the OpenShift cluster under test separately. 

## Troubleshooting Helper

Check also the documents below that might help while investigating the results and failures of the validation process:

- [Troubleshooting Guide](./troubleshooting-guide.md)
- [Installation Review](./user-installation-review.md)

## Feedback <a name="feedback"></a>

If you are a community user and found any bugs or issues, you can open a [new GitHub issue](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/issues/new).

If you are under validation process and are looking for guidance or feedback, please reach out to your Red Hat Partner Manager to assist you with the conformance process.