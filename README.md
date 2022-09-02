# OpenShift Provider Certification Tool

OpenShift Provider Certification Tool is used to evaluate an OpenShift installation on a provider or hardware is in conformance

## Documentation

- [User Guide](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/blob/main/docs/user.md)
- [Development Guide](https://github.com/redhat-openshift-ecosystem/provider-certification-tool/blob/main/docs/dev.md)

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
