# FAQ for OPCT

## What is the OpenShift Provider Compatibility Tool (OPCT)?

OPCT is designed to orchestrate conformance workflows on OpenShift/OKD
clusters, providing readiness signals to ones willing to validate
custom OpenShift installations.

OPCT was initially designed to validate the custom OpenShift installations
in non-integrated providers (a.k.a [agnostic installation](https://docs.openshift.com/container-platform/latest/installing/installing_platform_agnostic/installing-platform-agnostic.html),
with `platform.none: {}` option for `install-config.yaml`),
**orchestrating well-known e2e test suites** from Kubernetes and OpenShift ecosystem.

OPCT CLI was built on top of the utilities:

- [Sonobuoy](https://sonobuoy.io/) to orchestrate the test environment,
- [openshift-tests](https://github.com/openshift/origin#end-to-end-e2e-and-extended-tests) to orchestrate conformance test suites on OpenShift

The `openshift-tests` tool is a well-known tool used to orchestrate the conformance e2e suites across
the OpenShift/OKD CI jobs.

The `sonobuoy` tool is an official tool used to automate the Kubernetes certification environment to
be compliant with the [Kubernetes Certification Program](https://www.cncf.io/certification/software-conformance/),
providing many additional features, and totally extendable with Sonobuoy Plugins.

OPCT uses Sonobuoy aggregator server to orchestrate and manage the pipeline of the test workflow, and 
custom Sonobuoy Plugins, OPCT Plugins, to implement the workflow steps, orchestrating the OpenShift conformance tests
with `openshift-tests` utility.

OPCT CLI, client side tool, also implements several parsers to process the workflow results providing an analysis
of OpenShift/OKD specific features, best practices, requirements to a healthy environment based in previous results,
baseline archives/executions on well-known cloud providers to be used as reference deployments.

OPCT Plugins, workflow step, is Sonobuoy-compliant plugins to perform specific tasks. Currently the following plugins are implemented:

- `openshift-tests-plugin`: implement the step to setup test variants and orchestrate the conformance tests with `openshift-tests` utility. It implements interfaces to update progress to Sonobuoy server API by processing in real-time the output of `openshift-tests`, it also post-process JUnit to feed the workflow step pipeline to allow cross-step communication to perform customized tasks according to the previous results, such as replay failed tests in the previous conformance executions to get confidence while validating flake tests.
- `artifacts-collector`: is the last step responsible to collect required data to validate an OpenShift/OKD cluster. It is used as an 'artifact server' during workflow to plugins send data to be availalbe in the final artifact, such as test metadata, specific tests like FIO, conformance suite list, must-gather, metrics, etc

## What was OPCT not created for?

It's not created to:

* replace OpenShift CI tests/clusters tooling. OPCT CLI is a workflow orchestration, not
* implement component-specific e2e tests for OpenShift/OKD. Use custom Plugin instead.
* implement cloud Provider- specific e2e test. Use custom Plugin instead

## Is it possible to write custom e2e tests from my custom component in OPCT project?

Possibly yes, but it's not recommended and not supported by OPCT. The
OPCT was designed to implement the requirements of VSCP/OpenShift Provider Certification Program. The e2e tests
written as a plugin on OPCT are specific for the certification, or evaluate
specific install requirements on OpenShift cluster installed on external/non-supported platforms.

You have those options to implement e2e tests that should be tested on OpenShift installation:

* add the tests on the [openshift-tests utility](https://github.com/openshift/origin#end-to-end-e2e-and-extended-tests)
* if you would like to create a Sonobuoy plugin to keep compatibility with the existing certification tools, you can start looking at the [example repository](https://github.com/vmware-tanzu/sonobuoy-plugins/tree/main/examples/e2e-skeleton)


## Is it possible to create custom workflows using OPCT CLI?

Yes. OPCT CLI implements the requirements of OpenShift/OKD conformance suites,
although OPCT CLI is backed by Sonobuoy engine, inheriting all its features accessing it by `opct sonobuoy` subcommand.

Please take a look at the Sonobuoy Plugin examples repositories to explore what you can build: https://github.com/vmware-tanzu/sonobuoy-plugins

As a quick start guide to implement a custom plugin, take a look in the following steps:

1. Create the Plugin manifest definition:

```yaml
--- # file my-plugin.yaml

```

2. Schedule and monitor the Plugin using sonobuoy engine:

```sh
# Schedule
RUN_OPTS="--dns-namespace=openshift-dns --dns-pod-labels=dns.operator.openshift.io/daemonset-dns=default"
opct sonobuoy run -p my-plugin.yaml ${RUN_OPTS}

# Monitor the execution
opct status -n sonobuoy
```

3. Collect the results

```sh
opct retrieve -n sonobuoy archive.tar.gz
```

4. Review the results:

```sh
opct results archive.tar.gz
```


## Is it possible to orchestrate conformance suites with `openshift-tests` directly?

Yes. You need to use the `openshift-tests` shipped in the OpenShift release image of the exactly
version of your target cluster.

Let's say, you've installed a OpenShift cluster 4.16.10, you must extract the utility from
the release image:

```sh
# Declare or change the following environment variables:
# export PULL_SECRET_FILE=path/to/pull-secret.json
# export VERSION=4.16.10
# export ARCH=$(uname -m)

RELEASE_IMAGE="quay.io/openshift-release-dev/ocp-release:${VERSION}-${ARCH}"
TESTS_IMAGE=$(oc adm release info --image-for=tests -a ${PULL_SECRET_FILE} ${RELEASE_IMAGE})

oc image extract $TESTS_IMAGE -a ${PULL_SECRET_FILE} \
    --file="/usr/bin/openshift-tests"

chmod u+x ./openshift-tests
```

Then perform the conformance tests, saving the results to a custom directory `/tmp/results`:

```sh
# Run build-in conformance suites
openshift-tests run openshift/conformance --junit-dir /tmp/results

# OR, run a custom test list: must be valid on 'openshift-tests', use 'run suite --dry-run' to begging with.
# The example below are getting random test 
cat << EOF >./my-tests.txt
$(./openshift-tests run openshift/conformance --dry-run | grep ^'"\[' | shuf | head -n1)
EOF

openshift-tests run -f ./my-tests.txt --junit-dir /tmp/results
```
