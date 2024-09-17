# opct adm parse-metrics

Setup "dedicated node", the node required by OPCT to run the test environment.

The setup includes:
- Create the node label `node-role.kubernetes.io/tests`
- Set `NoSchedule` taints to the node

Optionally, you can leave the command to select the node to you, when `--name` is not set.

## Usage


## Examples

- Let CLI to select a node, which isn't running Prometheus pods:

```sh
opct adm setup-node -y
```
