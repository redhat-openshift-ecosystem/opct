# `opct run`

`opct run` starts the conformance test workflow in the target cluster.

Launches the provider validation environment inside of an already running OpenShift cluster.

## Usage

```sh
opct run [flags]
```

## Options

```txt
--8<-- "docs/assets/output/opct-run.txt"
```

### Global Flags

- `--kubeconfig string`: kubeconfig for target OpenShift cluster
- `--log-level string`: logging level (default "info")

## Summary

The `opct run` command is used to launch the provider validation environment inside an already running OpenShift cluster. It supports various flags to customize the execution, including developer mode options, image repository settings, run modes, and timeout configurations.

## Examples

```sh
# Run the conformance test environment with default settings
opct run

# Run the conformance test environment watching the execution
opct run --watch

# Run with a specific image repository
opct run --image-repository mirror.repository.net/opct

# Run in upgrade mode with a target OpenShift release image
opct run --mode upgrade --upgrade-to-image <image>
```

For more detailed usage and options, refer to the validation guide.
