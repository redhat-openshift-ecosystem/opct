# opct report | development

This document describe development details about the report.

First of all, the report is the core component in the review process.
It will extract all the data needed to the review process, transform
it into business logic, aggregating common data, loading it to the
final report data which is consumed to build CLI and HTML report output.

The input data is the report tarball file, which should have all required data, including must-gather.

The possible output channels are:

- CLI stdout
- HTML report file: stored at `<output_dir>/opct-report.html` (a.k.a frontend)
- JSON dataset: stored at `<output_dir>/opct-report.json`
- Log files: stored at `<output_dir>/failures-${plugin}`
- Minimal HTTP file serveer serving the `<output_dir>` as root directory in TCP port 9090 

Overview of the flow:

``` mermaid
%%{init: {"flowchart": {"useMaxWidth": false}}}%%

sequenceDiagram
  autonumber
  Reviewer->>Reviewer/Output: 
  Reviewer->>OPCT/Report: ./opct report [opts] --save-to <output_dir> <archive.tar.gz>
  OPCT/Report->>OPCT/Archive: Extract artifact
  OPCT/Archive->>OPCT/Archive: Extract files/metadata/plugins
  OPCT/Archive->>OPCT/MustGather: Extract Must Gather from Artifact
  OPCT/MustGather->>OPCT/MustGather: Run preprocessors (counters, aggregators)
  OPCT/MustGather->>OPCT/Report: Data Loaded
  OPCT/Report->>OPCT/Report: Data Transformer/ Processor/ Aggregator/ Checks
  OPCT/Report->>Reviewer/Output: Extract test output files
  OPCT/Report->>Reviewer/Output: Show CLI output
  OPCT/Report->>Reviewer/Output: Save <output_dir>/opct-report.html/json
  OPCT/Report->>Reviewer/Output: HTTP server started at :9090
  Reviewer->>Reviewer/Output: Open http://localhost:9090/opct-report.html
  Reviewer/Output->>Reviewer/Output: Browser loads data set report.json
  Reviewer->>Reviewer/Output: Navigate/Explore the results
```

## Data Pipeline

### Collector

> TODO describe the data collected by plugins

### ELT (Extractor/Load/Transform)

> TODO: describe what data source is extracted, which package, what data is extracted, and how it is transformed in the "report"

### Viewer

There are two types of viewers, consuming the data sources:

- CLI Report
- HTML Report (served by HTTP server)

## View Frontend

### CLI

> TODO describe the CLI viewer

### Frontend

> TODO: detail about the frontend viewer construct.


## Explore the data

### Process many results (batch)

```bash
export RESULTS=( ocp414rc2_Azure_Azure-IPI_202309291531 ocp414rc2_Azure_Azure-IPI-tmpstg_202309300444 ); for RES in ${RESULTS[*]}; do
  echo "CREATING $RES";
  mkdir -pv /tmp/results-shared/$RES ;
  ~/opct/bin/opct-devel report --server-skip --save-to /tmp/results-shared/$RES $RES;
done
```

### Metrics

```bash
ARTIFACT_NAME=ocp414rc0_AWS_None_202309222127_sonobuoy_47efe9ef-06e4-48f3-a190-4e3523ff1ae0.tar.gz
# check if metrics has been collected
tar tf $ARTIFACT_NAME |grep artifacts_must-gather-metrics.tar.xz

# extract the metrics data
tar xf $ARTIFACT_NAME plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather-metrics.tar.xz
mkdir metrics
tar xfJ plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather-metrics.tar.xz -C metrics/

# check if etcd disk fsync metrics has been collected by server
zcat metrics/monitoring/prometheus/metrics/query_range-etcd-disk-fsync-db-duration-p99.json.gz | jq .data.result[].metric.instance 

# Install the utility asciigraph: 

# plot the metrics
METRIC=etcd-disk-fsync-db-duration-p99;
DATA=${PWD}/oci/metrics/monitoring/prometheus/metrics/query_range-${METRIC}.json.gz;
for INSTANCE in $(zcat $DATA | jq -r .data.result[].metric.instance ); do zcat $DATA | jq -r ".data.result[] | select(.metric.instance==\"$INSTANCE\").values[]|@tsv" | awk '{print$2}' |asciigraph -h 10 -w 100 -c "$METRIC - $INSTANCE" ; done
```