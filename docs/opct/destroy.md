# `opct destroy`

`opct destroy` tears down the conformance test environment in the target cluster.

## Options

```txt
--8<-- "docs/assets/output/opct-destroy.txt"
```

## Arguments

- `--force`: Force the destruction without confirmation.
- `--timeout`: Set a timeout for the destruction process.

## Summary

The `opct destroy` command is used to remove the conformance test environment from the specified cluster. This is useful for cleaning up resources after tests have been completed.

## Examples

```sh
# Destroy the test environment with confirmation
opct destroy

# Force destroy the test environment without confirmation
opct destroy --force

# Destroy the test environment with a timeout of 10 minutes
opct destroy --timeout 10m
```
