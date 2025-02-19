# Dev Guide - Release components

This guides describes how to release a new version of OPCT considering all the project dependencies.

## Creating container images for components

### Sonobuoy

Steps to check if Sonobuoy provides images to the target platform in the version used by OPCT:

1) Check the Sonobuoy version used by OPCT
```bash
$ go list -m github.com/vmware-tanzu/sonobuoy
github.com/vmware-tanzu/sonobuoy v0.57.1
```

2) Check the Sonobuoy images built for the version required by OPCT
```bash
$ skopeo list-tags docker://docker.io/sonobuoy/sonobuoy | jq .Tags | grep -i v0.57.1
 "amd64-v0.57.1",
  "arm64-v0.57.1",
  "ppc64le-v0.57.1",
  "s390x-v0.57.1",
  "v0.57.1",
  "win-amd64-1809-v0.57.1",
  "win-amd64-1903-v0.57.1",
  "win-amd64-1909-v0.57.1",
  "win-amd64-2004-v0.57.1",
  "win-amd64-20H2-v0.57.1",
```

3) [Bump the desired Sonobuoy version](https://github.com/redhat-openshift-ecosystem/opct/blob/main/hack/image-mirror-sonobuoy/mirror.sh#L9C27-L9C43)
in the script `mirror.sh` to mirror the Sonobuoy image to the OPCT image repository.

4) Run the mirror script to mirror and push images to the OPCT registry:
> (you must have permissions to quay.io/opct, otherwise you can hack it to push to yours)
```bash
make image-mirror-sonobuoy
```

### Plugins images

#### Development builds

Create images to test locally:

```bash
make images
```

To build images individually, you can use, for example for a single arch:

```sh
PLATFORMS=linux/amd64 make build-plugin-tests
```

Take a look into individual targets for each program in the [Makefile](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/Makefile).

The script responsible to build images locally is [`build.sh`](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/build.sh).
Get started there if you want to explore more about the build pipeline.

#### Production builds

Images for production are automatically created by the build
pipeline [ci.yaml](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/.github/workflows/ci.yaml)
when:

- a push in the `main` branch, the `latest` image will be published in the image registry
- a tag is created from any `release-v*`, the same tag will be published in the image registry

The production images are built by default for `linux/amd64` and `linux/arm64` in the same manifest
for the following components:

- Plugin [openshift-tests](https://quay.io/repository/opct/plugin-openshift-tests?tab=tags)
- Plugin [artifacts-collector](https://quay.io/repository/opct/plugin-artifacts-collector?tab=tags)
- Plugin [Must-gather-monitoring](https://quay.io/repository/opct/must-gather-monitoring?tab=tags)
