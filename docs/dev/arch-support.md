# OPCT Devel Guide - Architecture support

OPCT projects are split into different components. Each
component has its own build process and dependencies.

OPCT is divided into two main components:
- OPCT CLI: client side application to provision the test environment, collect, and generate report.
- Test environment: a group of tools running in server-side, target OpenShift cluster,
  which is limited to architectures supported by OPCT

The next steps will describe how to explore building the components,
we advise that this guide be followed only if support for a new architecture is added,
or you are curious to learn how the components are packed.

## Client

The client (OPCT CLI) is built in the following platforms:

- linux/amd64
- linux/arm64
- darwin/amd64
- darwin/arm64
- windows/amd64

### Adding support to a new platform

The OPCT command line interface is built in Go language. To add support for a new OS/architecture
you need to:

- 1) Check if the Go toolchain used by [the project][go-mod] can build to the target architecture:
```bash
curl -s https://raw.githubusercontent.com/redhat-openshift-ecosystem/opct/main/go.mod | grep ^go
go version
go tool dist list
```
- 2) Modify the [Makefile][makefile] to build to the new architecture
- 3) Upddate the [CI release pipeline][ci-pipeline-release] to upload the CLI when new release is created
- 4) Test it

```sh
make build-darwin-arm64
```

[go-mod]: https://github.com/redhat-openshift-ecosystem/opct/blob/main/go.mod
[makefile]: https://github.com/redhat-openshift-ecosystem/opct/blob/main/Makefile
[ci-pipeline-release]: https://github.com/redhat-openshift-ecosystem/opct/blob/main/.github/workflows/go.yaml

## Server-side components

The following components are used by OPCT on the server side:

- Sonobuoy Aggregator Server
- Sonobuoy Worker
- Plugin openshift-tests
    - Must-Gather
    - Must-Gather Monitoring
    - etcdfio
    - camgi
    - openshift client
    - sonobuoy client
    - jq

### Server-side platforms

The components are built in the following platforms:

| Component | linux/amd64 | linux/arm64 | linux-s390x | linux-pp64le |
| -- | -- | -- | -- | -- |
| Sonobuoy Aggregator/Worker | yes | yes | yes | yes |
| Plugin openshift-tests | yes | yes | no | no |

### Supported platforms

OPCT can provide full feature coverage in the following platforms:

- linux/amd64
- linux/arm64

The regular execution can be done by running:

```bash
opct run --wait
```

### Limited platforms

In the other platforms, you should be able to run Kubernetes e2e tests
provided by Sonobuoy on the following platforms:

- linux/pp64le
- linux/s390x

The following command allows you to run such tests:

```bash
opct sonobuoy run --sonobuoy-image quay.io/opct/sonobuoy:v0.5.0-alpha.3
```

### Adding support to a new platform

The first requirement to support the new server-side platform is to ensure OpenShift/OKD
can provide payloads for that platform.

Once OpenShift is supported, each OPCT server-side component must be built to create a
full support.

The following components are required:

- Sonobuoy Aggregator Server/Worker
- Plugin openshift-tests
    - Must-gather
    - Must-gather monitoring
    - openshift client
    - sonobuoy client
    - jq

The following components are optional:

- Plugin openshift-tests:
    - camgi
    - etcdfio

#### Building Sonobuoy Aggregator and Worker image

See the release steps for more details how to mirror [Sonobuoy images](./release.md).

#### Building Plugin openshift-tests

See the [build script][build-sh], and the [Containerfile][containerfile-plugin-otests].

See the [introduced multi-arch PR #51](https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/pull/51) for reference.

#### Building Must-Gather Monitoring

See the [build script][build-sh], and the [Containerfile][containerfile-mgm].

[build-sh]: https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/build.sh
[containerfile-otests]: https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/openshift-tests-provider-cert/Containerfile
[containerfile-mgm]: https://github.com/redhat-openshift-ecosystem/provider-certification-plugins/blob/main/must-gather-monitoring/Containerfile
