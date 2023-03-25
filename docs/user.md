# User Guide

Welcome to the user documentation for the OpenShift Provider Certification Tool (OPCT)!

The OpenShift Provider Certification Tool is used to evaluate an OpenShift installation on an infrastructure or hardware provider is in conformance.

> Note: This document is under `preview` release and it's in constant improvement.

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

This section describes the steps of the process when submiting the results to Red Hat Partner.
If the goal is not sharing the results to Red Hat, you can go to the next section.

Overview of the process to apply the results to the Red Hat Partner support case:

0. Install an OpenShift cluster on **the version desired** to be validated
1. Prepare the OpenShift cluster to run the validated environment
2. Download and install the OPCT
3. Run the OPCT
4. Monitor tests 
5. Gather results
6. Share the results archive with Red Hat Partner support case

It's not uncommon for specific tests to occasionally fail.  As a result, you may be asked by Support Engineers to repeat the process a few times depending on the results.

Finally, you will be asked to manually upgrade the cluster to the next minor release.

More detail on each step can be found in the sections further below.

## Prerequisites <a name="prerequisites"></a>

A Red Hat OpenShift 4 cluster must be [installed](https://docs.openshift.com/container-platform/latest/installing/index.html) before certification can begin. The OpenShift cluster must be installed on your infrastructure as if it were a production environment. Ensure that each feature of your infrastructure you plan to support with OpenShift is configured in the cluster (e.g. Load Balancers, Storage, special hardware).

The table below describes the OpenShift and OPCT versions supported for each release and features:

| OPCT [version](releases) | OCP Supported versions | OPCT Execution mode |
| -- | -- | -- |
| v0.3.x | 4.9, 4.10, 4.11, 4.12 | regular, upgrade |
| v0.2.x | 4.9, 4.10, 4.11 | regular |
| v0.1.x | 4.9, 4.10, 4.11 | regular |


[releases]:https://github.com/redhat-openshift-ecosystem/provider-certification-tool/releases

### Standard Environment <a name="standard-env"></a>

A dedicated compute node should be used to avoid disruption of the test scheduler. Otherwise, the concurrency between resources scheduled on the cluster, e2e-test manager (aka openshift-tests-plugin), and other stacks like monitoring can disrupt the test environment, leading to unexpected results, like the eviction of plugins or aggregator server (`sonobuoy` pod).

The dedicated node environment cluster size can be adjusted to match the table below. Note the differences in the `Dedicated Test` machine:

| Machine       | Count | CPU | RAM (GB) | Storage (GB) |
| ------------- | ----- | --- | -------- | ------------ |
| Bootstrap     | 1     | 4   | 16       | 100          |
| Control Plane | 3     | 4   | 16       | 100          |
| Compute       | 3     | 4   | 16       | 100          |
| Dedicated Test| 1     | 4   | 8        | 100          |

*Note: These requirements are higher than the [minimum requirements](https://docs.openshift.com/container-platform/latest/installing/installing_bare_metal/installing-bare-metal.html#installation-minimum-resource-requirements_installing-bare-metal) for a regular cluster (non-certification) in OpenShift product documentation due to the resource demand of the certification environment.*

#### Environment Setup: Dedicated Node <a name="standard-env-setup-node"></a>

The `Dedicated Node` is a normal worker with additional label and taints to run the OPCT environment.

The following requirements must be satisfied:

1. Choose one node with at least 8GiB of RAM and 4 vCPU
2. Label node with `node-role.kubernetes.io/tests=""` (certification-related pods will schedule to dedicated node)
3. Taint node with `node-role.kubernetes.io/tests="":NoSchedule` (prevent other pods from running on dedicated node)

> Note: *certification pods will automatically have node selectors and taint tolerations if you use the `--dedicated` flag.*

There are two options to accomplish this type of setup:

##### Option A: Command Line

```shell
oc label node <node_name> node-role.kubernetes.io/tests=""
oc adm taint node <node_name> node-role.kubernetes.io/tests="":NoSchedule
```

##### Option B: Machine Set

If you have support for OpenShift's Machine API then you can create a new `MachineSet` to configure the labels and taints. See [OpenShift documentation](https://docs.openshift.com/container-platform/latest/machine_management/creating-infrastructure-machinesets.html#binding-infra-node-workloads-using-taints-tolerations_creating-infrastructure-machinesets) on how to configure a new `MachineSet`. Note that at the time of certification testing, OpenShift's product documentation may not mention your infrastructure provider yet!

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

**Note**: The `MachineConfigPool` should be created only when the execution mode (`--mode`) is `upgrade`. If you are not running upgrade tests, please skip this section.

One `MachineConfigPool`(MCP) with the name `opct` must be created, selecting the dedicated node labels. The MCP must be paused, thus the node running the validation environment will not be restarted while the cluster is upgrading, avoiding disruptions to the Conformance results.

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

#### Testing in a Disconnected Environment <a name="disconnected-env-setup"></a>

The OpenShift Provider Certification Tool requires numerous images during the setup and execution
of tests.  See [User Installation Guide - Disconnected Installations](./user-installation-disconnected.md) for details 
on how to configure a mirror registry and how to run the OpenShift Provider Certification Tool to rely on the mirror 
registry for images.

### Privilege Requirements <a name="priv-requirements"></a>

A user with [cluster administrator privilege](https://docs.openshift.com/container-platform/latest/authentication/using-rbac.html#creating-cluster-admin_using-rbac) must be used to run the provider certification tool. You also use the default `kubeadmin` user if you wish.

## Install <a name="install"></a>

There are two options to install the provider certification tool: prebuilt binary and build from source.

### Prebuilt Binary <a name="install-bin"></a>

The provider certification tool is shipped as a single executable binary which can be downloaded from [the Project Releases page](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/releases). Choose the latest version and the architecture of the node (client) you will execute the tool, then download the binary.

The provider certification tool can be used from any system with access to API to the OpenShift cluster under test.


### Build from Source <a name="install-source"></a>

See the [build guide](../README.md#building) for more information.


## Usage <a name="usage"></a>


### Run provider certification tests <a name="usage-run"></a>

Requirements:
- You have set the dedicated node
- You have installed OPCT

#### Run the default execution mode (regular) <a name="usage-run-regular"></a>

- Create and run the certification environment (detaching the terminal):

```sh
openshift-provider-cert run 
```

#### Run the 'upgrade' mode <a name="usage-run-upgrade"></a>

The mode `upgrade` runs the OpenShift cluster updates to the 4.y+1 version, then the regular Conformance tests will be executed (Kubernetes and OpenShift). This mode was created to Validate the entire update process, and to make sure the target OCP release is validated on the Conformance tests.

> Note: If you will submit the results to Red Hat Partner Support, you must have Validated the installation on the initial release using the regular execution. For example, to submit the upgrade tests for 4.11->4.12, you must have submitted the regular tests for 4.11. If you have any questions, ask your Red Hat Partner Manager.

Requirements for running 'upgrade' mode:

- You have created the `MachineConfigPool opct`
- You have the OpenShift client locally (`oc`) - or have noted the Digest of the Target Release
- You must choose the next Release of Y-stream (`4.Y+1`) supported by your current release. (See [update graph](https://access.redhat.com/labs/ocpupgradegraph/update_path))

```sh
openshift-provider-cert run --mode=upgrade --upgrade-to-image=$(oc adm release info 4.Y+1.Z -o jsonpath={.image})
```

## Run Tests with the Disconnected Mirror registry<a name="usage-run-disconnected"></a>

Tests are able to be run in a disconnected environment through the use of a mirror registry.

Requirements for running tests with a disconnected mirror registry:

- Disconnected Mirror Image Registry created
- Private cluster Installed: https://docs.openshift.com/container-platform/latest/installing/installing_bare_metal/installing-restricted-networks-bare-metal.html
- You created a registry on your mirror host: https://docs.openshift.com/container-platform/latest/installing/disconnected_install/installing-mirroring-installation-images.html#installing-mirroring-installation-images


To run tests such that they use images hosted by the Disconnected Mirror registry:

~~~
./openshift-provider-cert-linux-amd64 run --image-repository ${TARGET_REPO}
~~~

#### Optional parameters for run <a name="usage-run-optional"></a>

- Create and run the certification environment and keep watching the progress:
```sh
openshift-provider-cert run -w
```

### Check status <a name="usage-check"></a>

```sh
openshift-provider-cert status 
openshift-provider-cert status -w # Keep watch open until completion
```


### Collect the results <a name="usage-retrieve"></a>

The certification results must be retrieved from the OpenShift cluster under test using:

```sh
openshift-provider-cert retrieve
openshift-provider-cert retrieve ./destination-dir/
```

### Check the results archive <a name="usage-results"></a>

You can see a summarized view of the results using:

```sh
openshift-provider-cert results retrieved-archive.tar.gz
```

### Submit the results archive <a name="submit-results"></a>

How to submit OpenShift Certification Test results:

- Log in to the [Red Hat Connect Portal](https://connect.redhat.com/login).
- Go to [`Support > My support tickets > Create Case`](https://connect.redhat.com/support/technology-partner/#/case/new).
- In the `Request Category` step, select `Product Certification`.
- In the `Product Selection` step, for the Product field, select `OpenShift Container Platform` and select the Version you are using.
- Click `Next` to continue.
- In the `Request Details` step, in the `Request Summary` field, specify `[VCSP] OpenShift Provider Certification Tool Test Results` and provide any additional details in the `Please add description` field.
- Click `Next` to continue.
- Click `Submit` when you have completed all the required information.
- A Product Certification case will be created, and please follow the instructions provided to add the test results and any other related material for us to review.
- Go to [`Support > My support tickets`](https://connect.redhat.com/support/technology-partner/#/case/list) to find the case and review status and/or to add comments to the case.

Required files to attach to a NEW support case:

- Attach the detailed Deployment Document describing how the cluster is installed in your Cloud Provider.
- Download, review and attach the [`user-installation-checklist.md`](./user-installation-checklist.md) to the case.
- Attach the `"retrieved-archive".tar.gz` result file to the case.


### Environment Cleanup <a name="usage-destroy"></a>

Once the certification process is complete and you are comfortable with destroying the test environment:

```sh
openshift-provider-cert destroy
```

You will need to destroy the OpenShift cluster under test separately. 

## Troubleshooting Helper

Check also the documents below that might help while investigating the results and failures of the Provider Certification process:

- [Troubleshooting Guide](./troubleshooting-guide.md)
- [Installation Review](./user-installation-review.md)

## Feedback <a name="feedback"></a>

If you have any feedback, bugs, or other issues with this OpenShift Certification Tool, please reach out to your Red Hat partner to assist you with the conformance process.

You may also open a [new GitHub issue](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/issues/new) for bugs but you are still encouraged to notify your Red Hat partner.
