# `opct get images`

`opct get images` retrieves the list of container images used in the conformance test environment in the target cluster.

## Options

```txt
--8<-- "docs/assets/output/opct-get-images.txt"
```

## Arguments

This command does not take any arguments.

## Summary

The `opct get images` command is used to list all the container images that are part of the conformance test environment. This is useful for mirror required images on disconnected validations.

## Examples

To retrieve the list of images:

```sh
opct get images
```

This will output a list of container images used in the conformance test environment.

