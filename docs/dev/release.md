# Dev Guide - Release components

This guides describes how to release a new version of OPCT considering all the project dependencies.


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

