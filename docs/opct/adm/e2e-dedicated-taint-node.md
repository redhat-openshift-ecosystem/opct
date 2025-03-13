# `opct adm e2e-dedicated taint-node`

`opct adm e2e-dedicated taint-node` manages the setup of the dedicated compute node in the cluster.

The `dedicated` node is allocated to be used by the services **to launch** and **manage**
the test environment (Aggregator server, workflow steps, etc). It is created
to prevent the scheduler from using that node by default, to avoid e2e disruption
workloads impacting the environment (drain the node, evict services, etc),
as well as making sure we have the required resources to host the conformance jobs.

The following configuration is added:

- Create the node label `node-role.kubernetes.io/tests`
- Set `NoSchedule` taints on the node

When allowing the command to select the node automatically, it avoids nodes hosting monitoring workloads, including Prometheus, preventing oversizing and disruptions in the test environment.

## Options

```txt
--8<-- "docs/assets/output/opct-adm-e2e-dedicated-taint-node.txt"
```

## Summary

The `opct adm e2e-dedicated taint-node` command sets up a node for the validation process by applying the required label and taint configuration to `NoSchedule`.

## Examples

```sh
# Select a node randomly and set it up
opct adm e2e-dedicated taint-node

# Specify a node to set up
opct adm e2e-dedicated taint-node --node <node-name>

# Run the command with confirmation
opct adm e2e-dedicated taint-node -y
```

## Flags

- `-h, --help`: Help for setup-node
- `--node string`: Node to set required label and taints
- `-y, --yes`: Confirm the setup without prompting

## Global Flags

- `--kubeconfig string`: Kubeconfig for target OpenShift cluster
- `--log-level string`: Logging level (default "info")
