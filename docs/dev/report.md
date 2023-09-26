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

## Frontend

> TODO: detail about the frontend construct.