# `opct results`

`opct results` is the summarized command to show the result for each job for standard kubernetes cluster.

## Options

```txt
--8<-- "docs/assets/output/opct-results.txt"
```
## Example Usage

To generate a summarized page with failed tests for each plugin, you can use the following command:

```sh
opct results --plugin <plugin-name>
```

> Replace `<plugin-name>` with the name of the plugin you want to check.

To display the results of a specific plugin, you can use the following command:

```sh
opct results --plugin <plugin-name>
```

To generate a dump of the results, you can use the following command:

```sh
opct results --mode dump
```
