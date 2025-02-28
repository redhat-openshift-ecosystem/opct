# `opct adm baseline`

Administrative commands to manipulate baseline results. Baseline results are used to compare the results of the validation tests. These are CI results from reference installations which are used to compare the results from custom executions targeting to infer persistent failures, helping to isolate:
- Flaky tests
- Permanent failures
- Test environment issues

## Options

```txt
--8<-- "docs/assets/output/opct-adm-baseline.txt"
```

## Usage

```sh
opct adm baseline [flags]
opct adm baseline [command]
```

## Available Commands

- `get`: Get a baseline result to be used in the review process.
- `indexer`: (Administrative usage) Rebuild the indexer for baseline in the backend.
- `list`: List all available baseline results by OpenShift version, provider, and platform type.
- `publish`: Publish a baseline result to be used in the review process.

## Flags

- `-h, --help`: Help for baseline

## Global Flags

- `--kubeconfig string`: Kubeconfig for target OpenShift cluster
- `--log-level string`: Logging level (default "info")
