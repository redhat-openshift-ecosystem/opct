# Developer Guide

This document is a guide for developers detailing the OPCT solution, design choices and the implementation references.

Table of Contents:

- [Release](#release)
- [Development Notes](#dev-notes)
    - [Command Line Interface](#dev-cli)
    - [Integration with Sonobuoy CLI](#dev-integration-cli)
    - [Sonobuoy Plugins](#dev-sonobuoy-plugins)
    - [Diagrams](#dev-diagrams)
    - [CLI Result filters](#dev-diagram-filters)
    - [Running Customized Plugins](#dev-running-custom-plugins)
    - [Project Documentation](#dev-project-docs)

## Release <a name="release"></a>

Releasing a new version of the tool is done automatically through [this GitHub Action](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/blob/main/.github/workflows/release.yaml)
which is run on new tags. Tags should the SemVer format. Example: v0.1.0, v0.1.0-alpha1 (...)

Tags should only be created from the `main` branch which only accepts pull-requests that pass through [this CI GitHub Action](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/blob/main/.github/workflows/go.yaml).

The container image for CLI is automatically published to the container registry [`quay.io/ocp-cert/opct`](https://quay.io/repository/ocp-cert/opct?tab=tags) by Github Action job `Build Container and Release to Quay` every new release.

Note that any version in v0.* will be considered part of the **preview release** of the tool.

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

### Sonobuoy Plugins <a name="dev-sonobuoy-plugins"></a>

OPCT is extended by Sonobuoy Plugins.

The Plugins source code is available on the project [provider-certification-plugins](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins).

### Diagrams <a name="diagrams"></a>

The diagrams are available under the page [Diagrams](./diagrams).

### CLI Result filters <a name="dev-diagram-filters"></a>

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


### Running Customized Plugins <a name="dev-running-custom-plugins"></a>

In some situations, you may need to modify the plugins that are run by the OPCT.
Running the OPCT with customized plugin manifests cannot be used for final validation of an OpenShift cluster!
If you find issues or changes that are needed to complete, please open a GitHub issue or reach out to your Red Hat contact assisting with validation process.

1. Export default plugins to local filesystem:
```
openshift-provider-cert assets /tmp
INFO[2022-06-16T15:35:29-06:00] Asset openshift-conformance-validated.yaml saved to /tmp/openshift-conformance-validated.yaml 
INFO[2022-06-16T15:35:29-06:00] Asset openshift-kube-conformance.yaml saved to /tmp/openshift-kube-conformance.yaml 
```
2. Make your edits to the exported YAML assets:
```
vi /tmp/openshift-kube-conformance.yaml
```
3. Launch the tool with customized plugin:
```
openshift-provider-cert run --plugin /tmp/openshift-kube-conformance.yaml --plugin /tmp/openshift-conformance-validated.yaml
```

### Project Documentation  <a name="dev-project-docs"></a>

The documentation is available in the directory `docs/`. You can render it as HTML using `mkdocs` locally - it's not yet published the HTML version.

To run locally you should be using `python >= 3.8`, and install the `mkdocs` running:

```
pip install hack/docs-requirements.txt
```

Then, under the root of the project, run:

```
mkdocs serve
```

Then you will be able to access the docs locally on the address: http://127.0.0.1:8000/
