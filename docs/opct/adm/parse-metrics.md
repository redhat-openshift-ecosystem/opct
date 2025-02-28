# opct adm parse-metrics

Process Prometheus metrics collected by artifacts collector step, plotting those as HTML charts.

## Options

```txt
--8<-- "docs/assets/output/opct-adm-parse-etcd-logs.txt"
```

## Usage

- Added to OPCT in release: v0.5.0-alpha.3
- Feature status: beta
- Command: `opct adm parse-metrics options`
 
Options:

- `--input`: Input metrics file. Example: metrics.tar.xz
- `--output`: Output directory. Example: /tmp/metrics

## Metrics collector

The metrics can be collected into two different ways:

- OPCT archive (version v0.5.3-alpha.3+)
- must-gather-monitoring utility


## Examples

### Plot the metrics charts collected by OPCT

1. Extract the must-gather-monitoring from OPCT archive

```bash
tar xfz archive.tar.gz plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather-metrics.tar.xz
```

2. Process the metrics generating charts

```bash
./opct adm parse-metrics \
    --input plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather-metrics.tar.xz \
    --output ./metrics
```

3. Open the metrics directory from your file to explore the charts.

- `metrics.html`: charts plotted with [go-echarts](https://github.com/go-echarts)
- `index.html`: charts plotted with [Plotly](https://plotly.com/javascript/)

To explore the full javascript features, use a HTTP file server to view the charts:

```bash
cd ./metrics && python -m http.server 9090
```

### Plot the metrics charts collected by must-gather-monitoring

1. Run must-gather-monitoring to collect the metrics

```bash
oc adm must-gather --image=quay.io/opct/must-gather-monitoring:v0.1.0 &&\
tar xfJ must-gather-metrics.tar.xz
```

2. Process the metrics generating charts

```bash
./opct adm parse-metrics \
    --input must-gather-metrics.tar.xz \
    --output ./metrics
```

### Plot the metrics natively from OPCT report

`opct report` command generates charts automatically when
the metrics is available and the report HTML is enabled.

- Generate the report:
```bash
./opct report archive.tar.gz --save-to ./report
```

- Open the HTML report in your browser at http://localhost:9090/metrics

Read more about `opct report` in the [documentation](../report.md).
