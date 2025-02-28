# `opct adm setup-node`

`opct adm setup-node` manages the baseline results by selecting a dedicated compute node in the cluster. It avoids nodes hosting monitoring or Prometheus workloads to prevent oversizing and disruptions in the test environment.

## Options

```txt
--8<-- "docs/assets/output/opct-adm-cleaner.txt"
```

## Summary

The `opct adm setup-node` command sets up a node for the validation process by applying the necessary labels and taints.

## Examples

```sh
# Select a node randomly and set it up
opct adm setup-node

# Specify a node to set up
opct adm setup-node --node <node-name>

# Run the command with confirmation
opct adm setup-node -y
```

## Flags

- `-h, --help`: Help for setup-node
- `--node string`: Node to set required label and taints
- `-y, --yes`: Confirm the setup without prompting

## Global Flags

- `--kubeconfig string`: Kubeconfig for target OpenShift cluster
- `--log-level string`: Logging level (default "info")
