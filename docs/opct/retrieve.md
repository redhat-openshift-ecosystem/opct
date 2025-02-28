# `opct retrieve`

The `opct retrieve` command is used to download the result artifact from the test environment.

## Options

```txt
--8<-- "docs/assets/output/opct-retrieve.txt"
```

## Summary

The `opct retrieve` command allows users to download the artifact generated during the conformance test process. The artifact can include logs, reports, and other relevant files necessary for reviewing test results.

The file `opct_${uuid}.tar.gz` will be saved locally.

## Examples

To download an artifact to the current directory, use the following command:

```sh
opct retrieve
```
