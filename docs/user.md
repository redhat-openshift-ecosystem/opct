# OpenShift Provider Certification Tool - User Guide

Welcome to the user documentation for the OpenShift Provider Certification Tool!  

The OpenShift Provider Certification Tool is used to evaluate an OpenShift installation on an infrastructure or hardware provider is in conformance.

> Note: This document is under `preview` release and it's in constant improvement.

Table Of Contents:

- [Process](#process)
- [Prerequisites](#prerequisites)
    - [Standard Environment](#standard-env)
        - [Environment Setup](#standard-env-setup)
    - [Privilege Requirements](#priv-requirements)
- [Install](#install)
    - [Prebuilt Binary](#install-bin)
    - [Build from Source](#install-source)
- [Usage](#usage)
    - [Run provider certification tests](#usage-run)
    - [Check status](#usage-check)
    - [Collect the results](#usage-retrieve)
    - [Check the Results](#usage-results)
    - [Submit the Results](#submit-results)
    - [Environment Cleanup](#usage-destroy)
- [Troubleshooting](#troubleshooting)
- [Feedback](#feedback)

## Process <a name="process"></a>

More detail on each step can be found in the sections further below.

1. Prepare the OpenShift cluster to be certified
2. Download and install the provider certification tool
3. Run provider certification tool
4. Monitor tests 
5. Gather results
6. Share certification results and [must gather](https://docs.openshift.com/container-platform/latest/support/gathering-cluster-data.html) with Red Hat


## Prerequisites <a name="prerequisites"></a>

A Red Hat OpenShift 4 cluster must be [installed](https://docs.openshift.com/container-platform/latest/installing/index.html) before certification can begin. The OpenShift cluster must be installed on your infrastructure as if it were a production environment. Ensure that each feature of your infrastructure you plan to support with OpenShift is configured in the cluster (e.g. Load Balancers, Storage, special hardware).

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

#### Environment Setup <a name="standard-env-setup"></a>

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

- Run the certification environment in the background:
```sh
openshift-provider-cert run 
```

- Run the certification environment in the background and keep watching the progress:
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

<!-- > Option 1 - Send the results using Red Hat Connect Portal -->

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

- Attach the Installation Document used describing how your installing the Cluster
- Download, review and attach the [`user-installation-checklist.md`](./user-installation-checklist.md)
- Attach the `retrieved-archive.tar.gz` result file to the case.
- Attach the `must-gather.tar.gz` file to the case.


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
