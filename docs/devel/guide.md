# Developer Guide

This document is a guide for developers detailing the OPCT solution, design choices and the implementation references.

Table of Contents:

- [Create Releases](#release)
- [Development Notes](#dev-notes)
    - [Command Line Interface](#dev-cli)
    - [Integration with Sonobuoy CLI](#dev-integration-cli)
    - [Mirror sonobuoy images](#dev-image-mirror-sonobuoy)
- [Sonobuoy Plugins](#dev-sonobuoy-plugins)
- [Diagrams](#dev-diagrams)
- [CLI Result filters](#dev-diagram-filters)
- [Running Customized Plugins](#dev-running-custom-plugins)
- [Project Documentation](#dev-project-docs)

## Create Release <a name="release"></a>

Releasing a new version of the tool is done automatically through [GitHub Action workflow](https://github.com/redhat-openshift-ecosystem/opct/blob/main/.github/workflows/release.yaml)
which is run when tags are created. Tags should follow the [Semantic Versioning (SemVer)](https://semver.org/) standard. Example: v0.1.0, v0.1.0-alpha1 (...).

Tags should only be created from the `main` branch which only accepts pull-requests that pass through [this CI GitHub Action](https://github.com/redhat-openshift-ecosystem/opct/blob/main/.github/workflows/go.yaml).

The container image for CLI is automatically published to the container registry [`quay.io/opct/opct`](https://quay.io/repository/opct/opct?tab=tags) by Github Action job `Build Container and Release to Quay` every new release.

Note that any version in v0.* will be considered part of the **preview release** of the tool.

Release process checklist:

- Make sure the base image has Security issues on Quay Security Scan. Steps to check:
    - Build a "dev tag" for Plugin image: `cd openshift-tests-provider-cert && make build-dev`
    - Check the Security Scan results on https://quay.io/repository/opct/opct?tab=tags
        - if there is security scan needed to fix on the base image, you must build the base image and tools image with the following steps:
        - 1. bump the version with fixes on the base image. Example: [provider-certification-plugins#41](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/pull/41)
        - 2. build the tools image: `./hack/build-image.sh build-tools`
        - 3. push the tools image:  `podman push quay.io/opct/tools:v0.0.0-<the version>`
        - 4. check security scan results
        - 5. if success, promote to latest: `podman push quay.io/opct/tools:latest`
        - 6. rebuild the plugins image with new base
    - Build a "dev tag" for CLI image:
        - `make linux-amd64-container IMG=quay.io/my-user/opct`
        - `podman push quay.io/my-user/opct:latest`
    - Check the Security Scan results on https://quay.io/repository/my-user/opct?tab=tags
- Run one-shot/preflight before promoting the tag:
    - [install a cluster using the **latest OCP GA** version](https://redhat-openshift-ecosystem.github.io/opct/user/#prerequisites) for validation (platform agnostic installation)
    - [run the conformance validation (regular or upgrade)](https://redhat-openshift-ecosystem.github.io/opct/user/#run-conformance-tests)
    - [collect the results](https://redhat-openshift-ecosystem.github.io/opct/user/#collect-the-results)
    - review if the execution finished correctly
        - plugins finished
        - artifacts collected
        - results are under regular [CI executions](https://openshift-provider-certification.s3.us-west-2.amazonaws.com/index.html)
- Create a tag on [Plugins repository](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins) based on the `main` branch (or the commit for the release);
~~~bash
# Example
git tag v0.4.0 -m "Release for OPCT v0.4 related to features on OPCT-XXX"
git push --tags upstream
~~~
- Open a PR updating the [`PluginsImage` value](https://github.com/redhat-openshift-ecosystem/opct/blob/main/pkg/types.go#LL16C2-L16C14) on the CLI repository, merge it;
- Create a tag on [CLI/Tool repository](https://github.com/redhat-openshift-ecosystem/opct) based on the `main` branch (or the commit for the release)
~~~bash
# Example
git tag v0.4.0 -m "Release for OPCT v0.4 related to features on OPCT-XXX"
git push --tags upstream
~~~

### Manual tests

- Create an OCP cluster
- Prepare the cluster to run OPCT: set tests label for dedicated node, taint, create registry, create MachineConfigPool for upgrade, wait to be ready, etc
    - It's possible to use the Ansible Playbook to do everything in Day-2 by running (WIP on [#38](https://github.com/redhat-openshift-ecosystem/opct/pull/38)):
```bash
ansible-playbook hack/opct-runner/opct-run-tool-preflight.yaml  -e cluster_name=opct-v040
```
- Run the tool
```bash
./opct-linux-amd64 run -w --plugins-image=openshift-tests-provider-cert:v0.4.0-beta2
```
- Collect the results and compare with old releases. Baseline CI artifacts is available [here](https://openshift-provider-certification.s3.us-west-2.amazonaws.com/index.html).

## Development Notes <a name="dev-notes"></a>

This tool builds heavily on 
[Sonobuoy](https://sonobuoy.io) therefore at least
some high level knowledge of Sonobuoy is needed to really understand this tool. A 
good place to start with Sonobuoy is [its documentation](https://sonobuoy.io/docs).

OPCT extends Sonobuoy in two places:

- Command line interface (CLI)
- Plugins

### Command Line Interface <a name="dev-cli"></a>

Sonobuoy provides its own CLI but it has a considerable number of flags and options
which can be overwhelming. This isn't an issue with Sonobuoy, it's just the result
of being a very flexible tool. However, for simplicity sake, the OPCT extends
Sonobuoy CLI with some strong opinions specific to the realm validating
OpenShift on new infrastructure.

#### Integration with Sonobuoy CLI <a name="dev-integration-cli"></a>

The OPCT's CLI is written in Golang so that extending Sonobuoy is easily done.
Sonobuoy has two specific areas on which we build on:

- Cobra commands (e.g. [sonobuoy run](https://github.com/vmware-tanzu/sonobuoy/blob/87e26ab7d2113bd32832a7bd70c2553ec31b2c2e/cmd/sonobuoy/app/run.go#L47-L62))
- Sonobuoy Client ([source code](https://github.com/vmware-tanzu/sonobuoy/blob/87e26ab7d2113bd32832a7bd70c2553ec31b2c2e/pkg/client/interfaces.go#L246-L250))

Ideally, the OPCT's commands will interact with the Sonobuoy Client API. There may be some
situations where this isn't possible and you will need to call a Sonobuoy's Cobra Command directly. Keep in mind,
executing a Cobra Command directly adds some odd interaction; this should be avoided since the ability to cleanly \
set Sonobuoy's flags may be unsafe in code like below. The code below won't fail at compile time if there's a change
in Sonobuoy and there's also no type checking happening:

```golang
// Not Great
runCmd.Flags().Set("dns-namespace", "openshift-dns")
runCmd.Flags().Set("kubeconfig", r.config.Kubeconfig)
```

Instead, use the Sonobuoy Client includes with the project like this:

```golang
// Great
reader, ec, err := config.SonobuoyClient.RetrieveResults(&client.RetrieveConfig{
    Namespace: "sonobuoy",
    Path:      config2.AggregatorResultsPath,
})
```

#### Sonobuoy image mirroring <a name="dev-image-mirror-sonobuoy"></a>

The Sonobuoy images for Aggregator server and worker are mirrored to quay.io to prevent
issues with docker hub network throttling.

The Sonobuoy image must follow the same of OPCT CLI (sonobuoy library defined in go.mod).

**Running the mirror**

The mirror steps must be executed every time the Sonobuoy package is update on CLI, the following steps describes how to start the mirror process:

- Update the version `SONOBUOY_VERSION` in [`hack/image-mirror-sonobuoy/mirror.sh`][image-mirror-sonobuoy]
- Run mirror script
```bash
make image-mirror-sonobuoy
```
- Check the image in [quay.io/opct/sonobuoy](https://quay.io/repository/opct/sonobuoy?tab=tags)

**Running the mirror targeting custom repository**

```bash
SONOBUOY_VERSION=v0.56.11 MIRROR_REPO=quay.io/mrbraga/sonobuoy make image-mirror-sonobuoy
```

**Adding new Architecture**

If you are looking to test a new platform, you must add it when creating the manifest in [`hack/image-mirror-sonobuoy/mirror.sh`][image-mirror-sonobuoy]:

```bash
BUILD_PLATFORMS+=( ["windows-amd64"]="windows/amd64" )
```

**Testing custom sonobuoy images in unsupported architectures**

```bash
~/opct/bin/opct-devel run -w \
    --sonobuoy-image quay.io/mrbraga/sonobuoy:v0.56.12-linux-arm64 \
    --plugins-image openshift-tests-provider-cert:v0.5.0-alpha.3
```

[image-mirror-sonobuoy]: https://github.com/redhat-openshift-ecosystem/opct/tree/main/hack/image-mirror-sonobuoy/mirror.sh


## Sonobuoy Plugins <a name="dev-sonobuoy-plugins"></a>

OPCT is extended by Sonobuoy Plugins.

The Plugins source code is available on the project [provider-certification-plugins](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins).

## Diagrams <a name="diagrams"></a>

The diagrams are available under the page [Diagrams](./diagrams).

## CLI Result filters <a name="dev-diagram-filters"></a>

The CLI currently implements a few filters to help the reviewers (Partners, Support, Engineering teams) to find the root cause of the failures. The filters consumes the data sources below to improve the feedback, by plugin level, when using the command `process`:

- A. `"Provider's Result"`: This is the original list of failures by the plugin available on the command `results`
- B. `"Suite List"`: This is the list of e2e tests available on the respective suite. For example: plugin `openshift-kubernetes-conformance` uses the suite `kubernetes/conformance`
- C. `"Baseline's Result"`: This is the list of e2e tests that failed in the baseline provider. That list is built from the same Certification Environment (OCP Agnostic Installation) in a known/supported platform (for example AWS and vSphere). Red Hat has many teams dedicated to reviewing and improving the thousands of e2e tests running in CI, that list is constantly reviewed for improvement to decrease the number of false negatives and help to look for the root cause.
- D. `"Sippy"`: Sippy is the system used to extract insights from the CI jobs. It can provide individual e2e test statistics of failures across the entire CI ecosystem, providing one picture of the failures happening in the provider's environment. The filter will check for each failed e2e if has an occurrence of failures in the version used to be validated.

Currently, this is the order of filters used to show the failures on the `process` command:

-    `A intersection B` -> `Filter1`
- `Filter1 exclusion C` -> `Filter2`
- `Filter2 exclusion D` -> `Filter3`

The reviewers should look at the list of failures in the following order:

- `Filter3`
- `Filter2`
- `Filter1`
- `A`

The diagram visualizing the filters is available on draw.io, stored on the shared Google Driver Storage, needing one valid Red Hat account to access it (we have plans to make it public soon):
- https://app.diagrams.net/#G1NOhcF3jJtE1MjWCtbVgLEeD24oKr3IGa


## Running Customized Plugins <a name="dev-running-custom-plugins"></a>

See [BYO Plugin](./byo-plugin.md).

## Project Documentation  <a name="dev-project-docs"></a>

The documentation is available in the directory `docs/`. You can render it as HTML using `mkdocs` locally - it's not yet published the HTML version.

To run locally you should be using `python >= 3.8`, and install the `mkdocs` running:

```sh
pip install docs/requirements.txt
```

Then, under the root of the project, run:

```
mkdocs serve
```

Then you will be able to access the docs locally on the address: http://127.0.0.1:8000/


## e2e-dedicated controller

The e2e-dedicated controller is used to watch pods failing
to schedule and apply tolerations to allow scheduling in
the OPCT dedicated node, preventing false positives caused
by the required OPCT environment (dedicated worker node).

### Checking Logs in a Live Cluster

You can check the logs in a live cluster using the following command:

```sh
oc logs -n opct -l app=opct-dedicated-e2e-controller -f
```

### Inspecting the Logs in the Archive File

Alternatively you can inspect the controller logs from the archive
collected in the artifacts collector step:

```sh
mkdir results-raw results-must-gather
result_file=opct_202503132124_1899a26c-1564-4eab-b51a-2a39ca6caee4.tar.gz
mustgather_file=results-raw/plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather.tar.xz

# Extract the raw results
tar xfz ${result_file} -C results-raw

# Extracts the must-gather
tar xfJ ${mustgather_file} -C results-must-gather/

# Inspect the logs
less results-must-gather/must-gather-opct/inspect-opct/namespaces/opct/pods/opct-dedicated-e2e-controller-*/controller/controller/logs/current.log
```
