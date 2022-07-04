# OpenShift Provider Certification Tool

Welcome to the user documentation for the OpenShift Provider Certification Tool!  

The OpenShift Provider Certification Tool is used to evaluate an OpenShift installation on an infrastructure or hardware provider is in conformance.

> Note: This document is under `preview` release and it's constantly improvement.

Table Of Contents:

- [Process](#process)
- [Prerequisites](#prerequisites)
  - [Standard Environment](#standard-env)
  - [Dedicated test Environment](#dedicated-env)
    - [Environment Setup](#dedicated-env-setup)
  - [Privilege Requirements](#priv-requirements)
- [Install](#install)
  - [Prebuilt Binary](#install-bin)
  - [Build from Source](#install-source)
- [Usage](#usage)
  - [Run provider certification tests](#usage-run)
  - [Check status](#usage-check)
  - [Collect the results](#usage-retrieve)
  - [Check the Results](#usage-results)
  - [Environment Cleanup](#usage-destroy)
- [Certification Failures](#review)
  - [Troubleshooting](#review-troubleshooting)
    - [Review Results Archive](#review-archive)
    - [Do I Need a Dedicated Test Environment](#review-needed-dedicated)
    - [Cluster Failures](#review-cluster-failures)
- [Feedback](#feedback)

## Process <a name="process"></a>

More detail on each step can be found in sections further below. 

1. Prepare OpenShift cluster to be certified
2. Download and install provider certification tool
3. Run provider certification tool
4. Monitor tests 
5. Gather results
6. Share results with Red Hat


## Prerequisites <a name="prerequisites"></a>

A Red Hat OpenShift 4 cluster must be [installed](https://docs.openshift.com/container-platform/latest/installing/index.html) before certification can begin. The OpenShift cluster must be installed on your infrastructure as if it were a production environment. Ensure that each feature of your infrastructure you plan to support with OpenShift is configured in the cluster (e.g. Load Balancers, Storage, special hardware).

### Standard Environment <a name="standard-env"></a>

A standard machine layout can be used for certification. If you run into issues with pod disruption (eviction, OOM, frequent crashes, etc) then you may want to consider the Dedicated Test Environment configuration further below. Below is a table of the minimum resource requirements for the OpenShift cluster under test:

| Machine       | Count | CPU | RAM (GB) | Storage (GB) |
| ------------- | ----- | --- | -------- | ------------ |
| Bootstrap     | 1     | 4   | 16       | 100          |
| Control Plane | 3     | 4   | 16       | 100          |
| Compute       | 3     | 4   | 16       | 100          |


*Note: These requirements are higher than the [minimum requirements](https://docs.openshift.com/container-platform/latest/installing/installing_bare_metal/installing-bare-metal.html#installation-minimum-resource-requirements_installing-bare-metal) in OpenShift product documentation due to the resource demand of the certification tests.*

### Dedicated Node for Test Environment <a name="dedicated-env"></a>

If your compute nodes are at or below minimum requirements, it is recommended to run the certification environment on one dedicated node to avoid disruption of the test scheduler. Otherwise the concurrency between resources scheduled on the cluster, e2e-test manager (aka openshift-tests-plugin), and other stacks like monitoring can disrupt the test environment, leading to unexpected results, like eviction of plugins or certification server (sonobuoy pod).

See the troubleshooting section on ways to identify you might need to use a dedicated node test environment below.

The dedicated node environment cluster size can be adjusted to match the table below. Note the differences in the `Dedicated Test` machine:

| Machine       | Count | CPU | RAM (GB) | Storage (GB) |
| ------------- | ----- | --- | -------- | ------------ |
| Bootstrap     | 1     | 4   | 16       | 100          |
| Control Plane | 3     | 4   | 16       | 100          |
| Compute       | 3     | 4   | 8        | 100          |
| Dedicated Test| 1     | 4   | 8        | 100          |

#### Environment Setup <a name="dedicated-env-setup"></a>

1. Choose one node with at least 8GiB of RAM and 4 vCPU
2. Label node with `node-role.kubernetes.io/tests=""` (certification related pods will schedule to dedicated node)
3. Taint node with `node-role.kubernetes.io/tests="":NoSchedule` (prevent other pods from running on dedicated node)

> Note: *certification pods will automatically have node-selectors and taint tolerations if you use the `--dedicated` flag.*

There are two options to accomplish this type of setup:

##### Option A: Command Line 

```shell
oc label node <node_name> node-role.kubernetes.io/tests=""
oc taint node <node_name> node-role.kubernetes.io/tests="":NoSchedule
```

##### Option B: Machine Set 

If you have support for OpenShift's Machine API then you can create a new `MachineSet` to configure the labels and taints. See [OpenShift documentation](https://docs.openshift.com/container-platform/latest/machine_management/creating-infrastructure-machinesets.html#binding-infra-node-workloads-using-taints-tolerations_creating-infrastructure-machinesets) on how to configure a new `MachineSet`. Note that at time of certification testing, OpenShift's product documentation may not mention your infrastructure provider yet! 

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

A user with [cluster administrator privilege](https://docs.openshift.com/container-platform/latest/authentication/using-rbac.html#creating-cluster-admin_using-rbac) must be used to run the provider certification tool. You also use the default kubeadmin user if you wish. 

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


### Environment Cleanup <a name="usage-destroy"></a>

Once the certification process is complete and you are comfortable with destroying the test environment:

```sh
openshift-provider-cert destroy
```

You will need to destroy the OpenShift cluster under test separately. 


## Certification Tests Failures <a name="review"></a>

Under any type of certification test failure, it is recommended to recreate the cluster under test. The certification tests check cluster metrics and logs which are persisted and this could impact subsequent certification tests.

If you already know the reason for a test failure then resolve the problem and re-run the provider certification tool again so a new certification archive is created.

If you are not sure why you have failed tests or if some of the tests fail intermittently, proceed with the troubleshooting steps below.

> Note: When runing the `preview` release of the certification tool, it's expected to have failed tests reported on the archive, we are working to improve the accuracy. If you are sure the failed tests reported on the archive is not related to your environment, feel free to contact your Red Hat partner to share the feedback.


### Troubleshooting <a name="review-troubleshooting"></a>

#### Review Results Archive <a name="review-archive"></a>

The results archive file can be used to identify certification test failures so you can address them in your cluster installation process you are attempting to certify. 

The result archive file follows the format of the backend used to run the certification environment: Sonobuoy.

First, extract it to the `results` directory:

```bash
tar xfz <timestamp>_sonobuoy_<execution_id>.tar.gz -C results/
```

Once extracted, the archive file is grouped in the following subdirectories:

```
results/
├── hosts
├── meta
├── plugins
├── podlogs
├── resources
├── servergroups.json
└── serverversion.json
```
- `hosts` provides the kubelet configuration and health check for each node on the cluster
- `meta` has the metadata collected from the cluster and certification environment
- `plugins` has the plugins definitions and results
- `podlogs` has the logs of pods used on the certification environment: server and plugins
- `resources` has all the manifests for all the resources cluster and namespace scoped.
- `servergroups.json` has the APIGroupList custom resource
- `serverversion.json` has the Kubernetes version

To start exploring the problems in the certification environment, you can start looking into the `podlogs` directory.

The file `results/plugins/<_plugin_name_>/sonobuoy_results.yaml` has the results for each test. If the test has been failed, you can see the reason on the field `.details.failure` and `.details.system-out`:

Using the [`yq` tool](https://github.com/mikefarah/yq) you filter the failed tests by running this command:

- Getting the test names that have been `failed` from plugin `openshift-kube-conformance`:

```bash
yq -r '.items[].items[].items[] | select (.status=="failed") | .name ' results/plugins/openshift-kube-conformance/sonobuoy_results.yaml
```

- Get the `.failure` field for job `[sig-arch] Monitor cluster while tests execute`:

```bash
yq -r '.items[].items[].items[] | select (.name=="[sig-arch] Monitor cluster while tests execute").details.failure ' results/plugins/openshift-kube-conformance/sonobuoy_results.yaml
```

#### Do I Need a Dedicated Test Environment <a name="review-needed-dedicated"></a>

When issues like this arise, you can see error events in the `openshift-provider-certification` namespace (`oc get events -n openshift-provider-certification`) or even missing plugin pods. Also, sometimes sonobuoy does not detect the issues ([SPLAT-524](https://issues.redhat.com/browse/SPLAT-524)) and the certification environment will run until the timeout, with unexpected failures.

#### Cluster Failures <a name="review-cluster-failures"></a>

If you run into issues where the certification pods are crashing or the command line tool is not working for some reason then troubleshooting the OpenShift cluster under test may be required. 

Using the _status_ command will provide a high level overview but more information is needed to troubleshoot cluster level issues. A [Must Gather](https://docs.openshift.com/container-platform/latest/support/gathering-cluster-data.html) from the cluster and Inspection of sonobuoy namespace is the best way to start troubleshooting:

```sh
oc adm must-gather
oc adm inspect openshift-provider-certification
```

Use the two archives created by the commands above to begin troubleshooting. The must gather archive provides a snapshot view into the whole cluster. The inspection archive will contain information about the openshift provider certification namespace only.

## Feedback <a name="feedback"></a>

If you have any feedback, bugs, or other issues with this OpenShift Certification Tool, please reach out to your Red Hat partner assisting you with the conformance process. 

You may also open a [new GitHub issue](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/issues/new) for bugs but you are still encouraged to notify your Red Hat partner.
