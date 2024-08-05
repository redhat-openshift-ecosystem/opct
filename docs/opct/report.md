# opct report

`opct report` is a command to getting started exploring the result archive.

This is the main command used to extract results of customized OPCT wokrflow orchestrated by Sonobuoy.

The command reads the archive/result file and:
- provide a summary of standard conformance suites required to validate an OpenShift/OKD installation
- extract must-gather information, such as: state of objects, logs
- extract counter of error patterns from workloads (must-gather)
- apply SLOs (Checks) created from results executed in well-known (reference) deployments
- report failed SLOs providing guidance of failed item aiming to help/guide the OpenShift/OKD installation
- export counters, and expose it to be used as indicator when reviewing an OpenShift/OKD installation
- build a local Web UI app allowing you to navigate trhoughout the results extracted from the archive, such ash: e2e test failure's logs, e2e test metadata, e2e test documentation, CAMGI report, metrics report and counters.

To begging with, see the `Usage` section.

## Usage

- Basic usage:
```sh
opct report <archive.tar.gz>
```

- Advanced usage exposing/serving the WebUI:

```sh
opct report <archive.tar.gz> --save-to /tmp/results --log-level=debug
```

## Examples

### Exploring results using CLI

> TODO provide guidance to expore the WebUI sections

- Extract results without serving Web UI explore:
```sh
opct report <archive.tar.gz> --save-to /tmp/results --skip-server
```

### Exploring results using WebUI (serving HTTP locally)

> TODO provide guidance to expore the WebUI sections

- Extract results without serving Web UI explore:
```sh
opct report <archive.tar.gz> --save-to /tmp/results --embed-data
```

### Exploring results locally

> TODO do we still need it? It is used by QE or when there are security restrictions in the reviewer side

- Extract results without serving Web UI explore:
```sh
opct report <archive.tar.gz> --save-to /tmp/results --embed-data=true
```

- Explore the processed results locally at `/tmp/results`

> TODO

- Alternatively open the report in your browser: file:///tmp/results/index.html

### Using Filters: Baseline

The 'Baseline' filter is used to refine the results by isolating issues which is also
happening in the resame OpenShift version, with similar instance type, of yours.

The 'Baseline' are divided into two filters:
- 'BaselineAPI' filter: the failed filter pipeline discovers the correct baseline result (matching OpenShift version, platform type, and provider) using an external service, which is continuous updated by regular OPCT CI executions. This filter is enabled automatically and you can see the results in the plugin summary `Filter Failed API`. To disable this filter to investigate issues, set the flag `--skip-baseline-api`.
- 'BaselineArchive' filter (deprecated): The 'BaselineArchive' filter is used to isolate issues from an archive that you previously downloaded from OPCT result artifact/storage (limited access). The 'BaselineAPI' filter is a replacement of this option. To force using it, disable the 'BaselineAPI' and force the execution to skip deprecation warnings by setting the flags when `--diff` is used: `--force --skip-baseline-api`

Examples:

- Standard `report` workflow (using 'BaselineAPI' filter automatically):

~~~sh
$ opct-devel report --save-to results --log-level=debug opct-devel_v0.5_opct_202408080626_9e622261-316d-454f-baee-c407722668a5.tar.gz

┌───────────────────────────────────────────┐
│ 20-openshift-conformance-validated: ❌    │
├───────────────────────────┬───────────────┤
│ Total tests               │ 3783          │
│ Passed                    │ 1574          │
│ Failed                    │ 16            │
│ Timeout                   │ 0             │
│ Skipped                   │ 2193          │
│ Filter Failed Suite       │ 14 (0.37%)    │
│ Filter Failed KF          │ 14 (0.37%)    │
│ Filter Replay             │ 13 (0.34%)    │
│ Filter Failed Baseline    │ 13 (0.34%)    │
│ Filter Failed Priority    │ 13 (0.34%)    │
│ Filter Failed API         │ 1 (0.03%)     │
│ Failures (Priotity)       │ 1 (0.03%)     │
│ Result - Job              │ failed        │
│ Result - Processed        │ failed        │
└───────────────────────────┴───────────────┘
~~~

- Skipping 'BaselineAPI' filter:

~~~sh
$ opct-devel report --save-to results --log-level=debug ~/opct/results/opct-devel_v0.5_opct_202408080626_9e622261-316d-454f-baee-c407722668a5.tar.gz --skip-baseline-api
INFO[2024-08-08T12:31:32-03:00] Creating report...
WARN[2024-08-08T12:31:32-03:00] THIS IS NOT RECOMMENDED: detected flag --skip-baseline-api, setting OPCT_DISABLE_FILTER_BASELINE=1 to skip the failure filter in the pipeline
DEBU[2024-08-08T12:31:32-03:00] Processing results

┌───────────────────────────────────────────┐
│ 20-openshift-conformance-validated: ❌    │
├───────────────────────────┬───────────────┤
│ Total tests               │ 3783          │
│ Passed                    │ 1574          │
│ Failed                    │ 16            │
│ Timeout                   │ 0             │
│ Skipped                   │ 2193          │
│ Filter Failed Suite       │ 14 (0.37%)    │
│ Filter Failed KF          │ 14 (0.37%)    │
│ Filter Replay             │ 13 (0.34%)    │
│ Filter Failed Baseline    │ 13 (0.34%)    │
│ Filter Failed Priority    │ 13 (0.34%)    │
│ Filter Failed API         │ 13 (0.34%)    │
│ Failures (Priotity)       │ 13 (0.34%)    │
│ Result - Job              │ failed        │
│ Result - Processed        │ failed        │
└───────────────────────────┴───────────────┘
~~~

- Using `BaselineArchive` filter:

~~~sh
$ opct-devel report --save-to results --log-level=debug ~/opct/results/opct-devel_v0.5_opct_202408080626_9e622261-316d-454f-baee-c407722668a5.tar.gz  --diff ~/opct/results/opct-devel_v0.5_aws-aws-202408022202_sonobuoy_d743d645-c08d-438f-ba09-0417e764dd18.tar.gz --force
INFO[2024-08-08T12:55:08-03:00] Creating report...
WARN[2024-08-08T12:55:08-03:00] DEPRECATED: --baseline/--diff flag should not be used and will be removed soon.
Baseline are now discovered and applied to the filter pipeline automatically.
Please remove the --baseline/--diff flags from the command.
Additionally, if you want to skip the BaselineAPI filter, use --skip-baseline-api=true.
DEBU[2024-08-08T12:55:08-03:00] Processing results

┌───────────────────────────────────────────┐
│ 20-openshift-conformance-validated: ✅    │
├───────────────────────────┬───────────────┤
│ Total tests               │ 3783          │
│ Passed                    │ 1574          │
│ Failed                    │ 16            │
│ Timeout                   │ 0             │
│ Skipped                   │ 2193          │
│ Filter Failed Suite       │ 14 (0.37%)    │
│ Filter Failed KF          │ 14 (0.37%)    │
│ Filter Replay             │ 13 (0.34%)    │
│ Filter Failed Baseline    │ 0 (0.00%)     │
│ Filter Failed Priority    │ 0 (0.00%)     │
│ Filter Failed API         │ 0 (0.00%)     │
│ Failures (Priotity)       │ 0 (0.00%)     │
│ Result - Job              │ failed        │
│ Result - Processed        │ passed        │
└───────────────────────────┴───────────────┘
~~~

### Development examples

- Using custom plugin images

```sh
VERSION=v0.0.0-devel-d4745f8
opct-devel destroy; opct-devel run -w --devel-limit-tests=10 --log-level=debug \
--plugins-image=quay.io/opct/plugin-openshift-tests:${VERSION} \
--collector-image=quay.io/opct/plugin-artifacts-collector:${VERSION} \
--must-gather-monitoring-image=quay.io/opct/must-gather-monitoring:${VERSION};

opct-devel retrieve
```
