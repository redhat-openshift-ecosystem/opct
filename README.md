# OpenShift Provider Certification Tool

OpenShift Provider Certification Tool is used to evaluate an OpenShift installation on a provider or hardware is in conformance

```shell
Usage:
  openshift-provider-cert [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  destroy     Destroy current Certification Environment
  help        Help about any command
  results     Summary of certification results archive
  retrieve    Collect results from certification environment
  run         Run the suite of tests for provider certification
  sonobuoy    Generate reports on your Kubernetes cluster by running plugins
  status      Show the current status of the certification tool

Flags:
  -h, --help                help for openshift-provider-cert
      --kubeconfig string   kubeconfig for target OpenShift cluster
  -v, --version             version for openshift-provider-cert

Use "openshift-provider-cert [command] --help" for more information about a command.
```

## Building

- Go 1.17+ is needed.
- To build openshift provider cert tool invoke `make`.
- Cross build make targets are also available for Windows and MacOS. See [Makefile](./Makefile) for more on this.

## Dependencies

Dependencies are managed through Go Modules. When updating any dependency the suggested workflow is:

```shell
go mod tidy
go mod vendor
```
