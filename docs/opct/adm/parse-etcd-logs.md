# opct adm parse-etcd-logs

Extract information from etcd logs from pods collected by must-gather.

## Options

```txt
--8<-- "docs/assets/output/opct-adm-parse-etcd-logs.txt"
```

## Usage

- Added to OPCT in release: v0.5.0-alpha.3
- Command: `opct adm parse-etcd-logs options args`

Options:

- `--aggregator`: choose aggregator (all, day, hour, minute). Default: hour
- `--skip-error-counter`: flag to skip the error counter calculation to a faster report. Default: false

Args:

- `path/to/must-gather/directory` (optional)

## Examples

- Read from stdin:

```bash
export MUST_GATHER_PATH=./must-gather
tar xfz must-gather.tar.gz -C ${MUST_GATHER_PATH}
cat ${MUST_GATHER_PATH}/*/*/namespaces/openshift-etcd/pods/*/etcd/etcd/logs/*.log |\
    opct adm parse-etcd-logs
```

- Parse a directory with must-gather:

```bash
opct adm parse-etcd-logs ${MUST_GATHER_PATH}
```

- Aggregate by day:

```bash
opct adm parse-etcd-logs --aggregator day ${MUST_GATHER_PATH}
```

- Ignore error counters:

```bash
opct adm parse-etcd-logs --skip-error-counter true ${MUST_GATHER_PATH} 
```
