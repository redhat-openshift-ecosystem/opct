
# Dev Guide - Updating OPCT components

OPCT project delivers the following components:

- OPCT CLI: client-side utility to orchestrate the conformance workflow. It is delivered by binary and Contianer image.
- Aggregator server: the aggregator server is the Sonobuoy service responsible to handle the status of the workflow steps, and aggregate the results. It is delivered as a Container image to a repository in OPCT registry, mirroring from the supported version by OPCT
- OPCT Plugins: Plugins are step definition used by the workflow created by CLI. The steps are Kubernetes Pod definitions delivered as container images in quay.io/opct repository, and manifest defined in the CLI. Currently the following steps have it's own build definitions, and are released together in the plugins monorepo:
    - openshift-tests plugin
    - must-gather-monitoring
    - collector
    - tools

The table below summarizes the components:

| Component Name | Source/Doc | Image |
| -- | -- | -- |
| opct (cli) | [source][opct-src] | [quay.io/opct/opct](https://quay.io/repository/opct/opct?tab=tags) |
| sonobuoy (mirror) | [mirror info][sb-mirror] | [quay.io/opct/sonobuoy](https://quay.io/repository/opct/sonobuoy?tab=tags) |
| utility: tools | [source][tools] | [quay.io/opct/tools](https://quay.io/repository/opct/opct?tab=tags) |
| plugin: openshift-tests | [source][pl-ot] | [quay.io/opct/plugin-openshift-tests](https://quay.io/repository/opct/plugin-openshift-tests?tab=tags) |
| plugin: artifacts-collector | [source][pl-ac] | [quay.io/opct/lugin-artifacts-collector](https://quay.io/repository/opct/plugin-artifacts-collector?tab=tags) |
| plugin: must-gather-monitoring | [source][pl-mgm] | [quay.io/opct/lugin-artifacts-collector](https://quay.io/repository/opct/must-gather-monitoring?tab=tags) |

[opct-src]: https://github.com/redhat-openshift-ecosystem/opct
[opct-repo]: https://quay.io/repository/opct/opct?tab=tags
[sb-mirror]: https://redhat-openshift-ecosystem.github.io/opct/dev/#sonobuoy-image-mirroring
[pl-ot]: https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/tree/main/openshift-tests-plugin
[pl-ac]: https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/tree/main/artifacts-collector
[pl-mgm]: https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/tree/main/must-gather-monitoring
[tools]: https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/build.sh#L60

The following sections describes the steps of how to release each component.

Table of Contents:

- Container Image OS version update
- Golang version update
- Container image `tools` - utilities update (layered image)
- Sonobuoy version update

## Container Image OS version update

The project standarize to use the same version across all images shipped by the project: `quay.io/fedora/fedora-minimal:${MAJOR}$`

When the `${MAJOR}` need to be changed, it's a practice to be changed across the following images:

- [OPCT cli](https://github.com/redhat-openshift-ecosystem/opct/blob/main/hack/Containerfile)
- Plugins monorepo:
    - [Tools tools/Containerfile](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/tools/Containerfile)
    - [openshift-tests-plugin openshift-tests-plugin/Containerfile](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/openshift-tests-plugin/Containerfile)
    - [must-gather-monitoring/Containerfile](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/must-gather-monitoring/Containerfile)
    - [artifacts collector artifacts-collector/Containerfile](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/artifacts-collector/Containerfile)


## Golang version update

Version update on CLI:

- Update the [go.mod](https://github.com/redhat-openshift-ecosystem/opct/blob/main/go.mod)
- Update the Go version on all [CI workflows](https://github.com/redhat-openshift-ecosystem/opct/tree/main/.github/workflows)
- Update the [Container image](https://github.com/redhat-openshift-ecosystem/opct/blob/main/hack/Containerfile)

Version update on Plugin `openshift-tests`:

- Update the [go.mod](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/openshift-tests-plugin/go.mod)
- Update the Go version on all [CI workflows](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/tree/main/.github/workflows)
- Update the Go version in the plugin's [Container image](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/openshift-tests-plugin/Containerfile)


## Container image `tools` - utilities update (layered image)

`tools` image handles utilities tools used by workflow steps (plugins),
currently there are three main dependencies:

- `oc` utility - OpenShift Client
- `jq` utility - used to parse JSON files
- `camgi` utility - utility to create unified report from must-gather

Those plugins are used by the images:

- [artifacts-collector](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/artifacts-collector/Containerfile#L8)
- [must-gather-monitoring](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/must-gather-monitoring/Containerfile#L5)

Steps to build the `tools` image:

- step 1) Update required utilitieis (eg, bump jq, oc, etc) for each dependency in `build.sh` script

- step 2) Update the static version of tools image:

> In general tools image are released by y-stream when utilities are changed.

```sh
echo "v0.5.0" > tools/VERSION
```

- step 3) Build and publish the image

> If you want to build locally only, remove the `COMMAND` variable

```sh
make build-tools-release COMMAND=push EXPIRE=never
```

- step 4) update the dependencies to new version


## Updating Sonobuoy version

[Sonobuoy][sb] is used by OPCT to orchestrate and aggregate the results of the test environment. The CLI wraps mostly sonobuoy CLI command customizing to OpenShift conformance environment.

The Sonobuoy is used as a library in the OPCT CLI (client-side), plugin `openshift-tests`, and in the server-side the aggregator. All components must use the same versions of Sonobuoy.

### Update the Sonobuoy library

Steps to update the Sonobuoy librar in the `openshift-tests` plugin:

- Edit the [go.mod](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/openshift-tests-plugin/go.mod)
- Update the library `github.com/vmware-tanzu/sonobuoy` version to [supported one][sb-version]
- Open a PR with your changes
- Merge it
- (optional) Create a [release](./release.md)
- Create a release (or just test it on CLI using command `run --plugins-image=your image`)

Steps to update the Sonobuoy library in CLI project:

- Edit the [go.mod](https://github.com/redhat-openshift-ecosystem/opct/blob/main/go.mod)
- Update the library `github.com/vmware-tanzu/sonobuoy` version to [supported one][sb-version]
- Update the [plugins image](https://github.com/redhat-openshift-ecosystem/opct/blob/main/pkg/types.go)
- Open a PR with your changes

[sb-versions]: https://github.com/vmware-tanzu/sonobuoy/releases
[sb]: https://sonobuoy.io/

### Mirror Sonobuoy aggregator server image

Steps to check if Sonobuoy provides images to the target platform in the version used by OPCT:

1) Check the Sonobuoy version used by OPCT
```bash
$ go list -m github.com/vmware-tanzu/sonobuoy
github.com/vmware-tanzu/sonobuoy v0.57.3
```

2) Check the Sonobuoy images built for the version required by OPCT
```bash
$ skopeo list-tags docker://docker.io/sonobuoy/sonobuoy | jq .Tags | grep -i v0.57.3
 "amd64-v0.57.3",
  "arm64-v0.57.3",
  "ppc64le-v0.57.3",
  "s390x-v0.57.3",
  "v0.57.3",
  "win-amd64-1809-v0.57.3",
  "win-amd64-1903-v0.57.3",
  "win-amd64-1909-v0.57.3",
  "win-amd64-2004-v0.57.3",
  "win-amd64-20H2-v0.57.3",
```

3) [Bump the desired Sonobuoy version](https://github.com/redhat-openshift-ecosystem/opct/blob/main/hack/image-mirror-sonobuoy/mirror.sh#L9C27-L9C43)
in the script `mirror.sh` to mirror the Sonobuoy image to the OPCT image repository.

4) Run the mirror script to mirror and push images to the OPCT registry:
> (you must have permissions to quay.io/opct, otherwise you can hack it to push to yours)
```bash
make image-mirror-sonobuoy
```
