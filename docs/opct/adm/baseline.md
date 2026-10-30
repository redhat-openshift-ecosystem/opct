# `opct adm baseline [actions]`

Administrative commands to manipulate baseline results.

Baseline results are conformance executions accepted for
use as reference results during the review process.

The baseline results are automated executions that are
automatically published to the result services.

The `report` command automatically consumes the latest valid result from a specific
`OpenShift Version` and `Platform Type` in the filter pipeline (`Failed Filter API`),
making inferences about common failures in that specific release which **may** not be directly
related to the environment being validated.

The filter helps to isolate:

- Flaky tests in CI environment
- Permanent failures in the platform and release tested
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

Overview of commands:

- `opct adm baseline list`: List available baselines.
- `opct adm baseline get`: Get a summary of a specific baseline.
- `opct adm baseline publish`: (restricted) Publish artifacts to the OPCT services.
- `opct adm baseline indexer`: (restricted) Rebuild the index of the report service to serve the latest baseline summary.

## Available Commands

- `get`: Get a baseline result to be used in the review process.
- `indexer`: (Administrative usage) Rebuild the indexer for baseline in the backend.
- `list`: List all available baseline results by OpenShift version, provider, and platform type.
- `publish`: Publish a baseline result to be used in the review process.

## Workflow-scoped baselines

Baseline results are scoped by **workflow** to prevent cross-workflow
contamination. For example, an upgrade baseline must not be used to filter
known failures in a regular (non-upgrade) conformance run, or vice versa.

### Accepted `--workflow` values

| Value | Description |
|-------|-------------|
| `regular` | The default conformance workflow (non-upgrade). |
| `upgrade` | The upgrade validation workflow. |
| *(empty)* | When omitted, the first reachable baseline is accepted regardless of workflow (legacy behavior). |

### How workflow matching works

When `--workflow` is specified (or when the `report` command discovers the
workflow from the archive's OPCT runtime configuration):

1. **Workflow-scoped paths** are tried first
   (e.g. `4.20_External_upgrade_latest.json`).
2. **Legacy paths** (e.g. `4.20_External_latest.json`) are tried as fallback.
   The payload is validated using all available metadata:
     - The explicit `setup.api.workflow` field (present in new payloads).
     - The `provider.runtime.opctConfig` `run-mode` entry (older payloads).
     - The `provider.isUpgradeWorkflow` flag (older upgrade payloads).
3. A payload whose inferred workflow **matches** the target is accepted.
4. A payload whose inferred workflow **differs** is rejected.
5. A payload with **no verifiable workflow metadata** is skipped. Unknown
   provenance does not authorize workflow selection.

### Listing and indexing

The `list` command now includes a **Workflow** column. Each combination of
release, platform, and workflow maintains an independent "latest" entry so
that publishing an upgrade baseline does not displace the conformance
baseline for the same release and platform.

The `indexer` command reads the `workflow` tag from each summary object's
`setup.api` metadata. Legacy objects without the tag keep an empty workflow
field and use the two-part latest key (`release_platform`), preserving
backward compatibility.

### Summary name format

Published summaries include the workflow in the S3 object key when present:

- With workflow: `4.20_External_20260101120000_upgrade.json`
- Without workflow (legacy): `4.20_External_20260101120000.json`

## Global Flags

- `-h, --help`: Help for baseline
- `--kubeconfig string`: Kubeconfig for target OpenShift cluster
- `--log-level string`: Logging level (default "info")

## Examples

- List the latest summary's artifacts by version and platform type:

```bash
$ opct adm baseline list
+------------------------+--------+---------+--------------+----------+--------------------------------------+
| ID                     | TYPE   | RELEASE | PLATFORMTYPE | WORKFLOW | NAME                                 |
+------------------------+--------+---------+--------------+----------+--------------------------------------+
| 4.17_None              | latest | 4.17    | None         | -        | 4.17_None_20250302205650             |
| 4.17_External          | latest | 4.17    | External     | -        | 4.17_External_20250303140431         |
| 4.20_External          | latest | 4.20    | External     | -        | 4.20_External_20260301032412         |
| 4.20_External_upgrade  | latest | 4.20    | External     | upgrade  | 4.20_External_20260301044656_upgrade |
+------------------------+--------+---------+--------------+----------+--------------------------------------+
```

The **Workflow** column shows the workflow scope of each latest entry.
A `-` indicates a legacy baseline that predates workflow-scoped publication.
Conformance and upgrade baselines maintain independent latest entries per
release and platform.

- Review the summary for a latest artifact from a specific release:

```bash
$ opct adm baseline get --platform=External --release=4.18

INFO Getting latest baseline result by release and platform: 4.18/External (workflow=)
INFO Baseline result processed from archive: 4.18.0-0.nightly-...
>> Example serializing and extracting plugin failures for  20-openshift-conformance-validated
[0]: [sig-instrumentation] ...
....
```

- Retrieve the latest **upgrade** baseline for a specific release:

```bash
$ opct adm baseline get --platform=External --release=4.20 --workflow=upgrade

INFO Getting latest baseline result by release and platform: 4.20/External (workflow=upgrade)
INFO Baseline result processed from archive: 4.20.0-0.nightly-...-upgrade.tar.gz
....
```

When `--workflow` is specified, only baselines whose workflow can be
positively verified are accepted. A legacy baseline with no workflow
metadata is skipped with a warning.

- Publish artifacts to the OPCT services (**administrative only**):

```bash
export PROCESS_FILES="4.15.0-rc.7-20240221-HighlyAvailable-vsphere-None.tar.gz
4.15.0-rc.7-20240221-HighlyAvailable-vsphere-External.tar.gz
4.15.0-rc.1-20240110-HighlyAvailable-vsphere-External.tar.gz
4.15.0-20240228-HighlyAvailable-vsphere-None.tar.gz
4.15.0-20240228-HighlyAvailable-vsphere-External.tar.gz"

# Upload each baseline artifact
for PF in $PROCESS_FILES;
do
    opct adm baseline publish --log-level=debug "$HOME/opct/$PF";
done

# re-index
opct adm baseline indexer

# Expire CloudFront cache if you received an error (manual step):
# - AWS Console: AWS CloudFront > Distributions > Select Distribution > Invalidations > Create new expiring '/*'
# - AWS CLI: $ aws cloudfront create-invalidation --distribution-id <id> --paths /*

# Check the latest baseline data
opct-devel adm baseline list --all

# check all baseline data
opct-devel adm baseline list
```
