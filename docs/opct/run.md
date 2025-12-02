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

# Run preflight checks only without creating resources (dry-run mode)
opct run --dry-run

# Print rendered plugin manifests to stdout for debugging
opct run --dry-run --verbose

# Short form of verbose flag
opct run --dry-run -v
```

## Additional Flags

### `--dry-run`

Runs preflight checks and validates the environment without creating any resources. This is useful for:
- Verifying cluster readiness before executing tests
- Checking image accessibility
- Validating cluster operator status
- Testing manifest rendering without execution

### `--verbose` / `-v`

Prints rendered plugin manifests to stdout. This flag helps developers:
- See how CLI arguments affect manifest rendering from templates
- Debug manifest generation issues
- Understand the final YAML configuration before execution
- Verify template variable substitution

Commonly used with `--dry-run` to inspect manifests without creating resources.

For more detailed usage and options, refer to the validation guide.
