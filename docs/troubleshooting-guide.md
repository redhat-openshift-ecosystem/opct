# OPCT - Troubleshooting Guide

- [Conformance Failures](#review)
- [Troubleshooting](#review-troubleshooting)
    - [Review Results Archive](#review-archive)
    - [Cluster Failures](#review-cluster-failures)

## Conformance Tests Failures <a name="review"></a>

Under any type of conformance test failure, it is recommended to recreate the cluster under test.
The conformance tests check cluster metrics and logs which are persisted and this **will impact subsequent conformance tests**.

If you already know the reason for a test failure then resolve the problem, re-install the cluster under test, and re-run the provider conformance tool again so a new archive is created.

If you are not sure why you have failed tests or if some of the tests fail intermittently, proceed with the troubleshooting steps below.

> Note: The OPCT is in constant development, and due to the dynamic of the e2e test, you may experience failed tests reported on the archive that could be a flake, we are working to improve the accuracy of the reports. If you are sure the failed tests reported on the archive are not related to your environment, feel free to contact your Red Hat partner to share the feedback.

## Troubleshooting <a name="review-troubleshooting"></a>

#### Review Results Archive <a name="review-archive"></a>

The results archive file can be used to identify test failures so you can address them in the cluster installation process you are attempting to validate.

The result archive file follows the format of the backend used to run the validation environment: Sonobuoy.

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
- `meta` has the metadata collected from the cluster and validation environment
- `plugins` has the plugins definitions and results
- `podlogs` has the logs of pods used on the validation environment: server and plugins
- `resources` has all the manifests for all the resources cluster and namespace scoped.
- `servergroups.json` has the APIGroupList custom resource
- `serverversion.json` has the Kubernetes version

To start exploring the problems in the validation environment, you can start looking into the `podlogs` directory.

The file `results/plugins/<_plugin_name_>/sonobuoy_results.yaml` has the results for each test. If the test has failed, you can see the reason in the field `.details.failure` and `.details.system-out`:

Using the [`yq` tool](https://github.com/mikefarah/yq) you filter the failed tests by running this command:

- Getting the test names that have been `failed` from plugin `openshift-kube-conformance`:

```bash
yq -r '.items[].items[].items[] | select (.status=="failed") | .name ' results/plugins/openshift-kube-conformance/sonobuoy_results.yaml
```

- Get the `.failure` field for job `[sig-arch] Monitor cluster while tests execute`:

```bash
yq -r '.items[].items[].items[] | select (.name=="[sig-arch] Monitor cluster while tests execute").details.failure ' results/plugins/openshift-kube-conformance/sonobuoy_results.yaml
```

#### Cluster Failures <a name="review-cluster-failures"></a>

If you run into issues where the conformance pods are crashing or the command line tool is not working for some reason then troubleshooting the OpenShift cluster under test may be required.

Using the _status_ command will provide a high-level overview but more information is needed to troubleshoot cluster-level issues. A [Must Gather](https://docs.openshift.com/container-platform/latest/support/gathering-cluster-data.html) from the cluster and Inspection of the sonobuoy namespace is the best way to start troubleshooting:

```sh
oc adm must-gather
oc adm inspect openshift-provider-certification
```

Use the two archives created by the commands above to begin troubleshooting. The must-gather archive provides a snapshot view of the whole cluster. The inspection archive will contain information about the `openshift-provider-certification` namespace only.
