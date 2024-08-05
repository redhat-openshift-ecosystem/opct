# opct adm baseline [actions]

Manage baseline artifacts on OPCT backend.

> Note: This is administrative task, if you are not managing OPCT backends, skip this document.

OPCT baseline artifacts are conformance executions that have been accepted to
be used as a reference results during the review process.

The baselines artifacts are automated executions that is automatically published to
the OPCT services.

The `report` command consumes automatically the latest valid result from an specific
`OpenShift Version` and `Platform Type` in the filter pipeline (`Failed Filter APIP`),
making the inference of common failure in that specific release which **may** not be directly
related with the environment that is validated.

To begging with, explore the Usage section.

## Usage

Commands:
- `opct adm baseline list`: List baselines available.
- `opct adm baseline get`: Get a specific baseline summary.
- `opct adm baseline publish`: (restricted) Publish artifacts to the OPCT services.
- `opct adm baseline indexer`: (restricted) Re-index the report service to serve the baseline summary.

## Examples

- List the latest summary's artifacts by version and platform type:

```bash
$ opct adm baseline list
+---------------+--------+-------------------+--------------+------------------------------+
| ID            | TYPE   | OPENSHIFT VERSION | PLATFORMTYPE | NAME                         |
+---------------+--------+-------------------+--------------+------------------------------+
| 4.15_External | latest | 4.15              | External     | 4.15_External_20240228043414 |
| 4.15_None     | latest | 4.15              | None         | 4.15_None_20240228041900     |
+---------------+--------+-------------------+--------------+------------------------------+
```

- List all available summary artifacts:

```bash
$ opct adm baseline list --all
+--------+---------+----------+----------+------------------------------+
| LATEST | VERSION | PLATFORM | PROVIDER | NAME                         |
+--------+---------+----------+----------+------------------------------+
|        | 4.15    | External | vsphere  | 4.15_External_20240110044423 |
|        | 4.15    | External | vsphere  | 4.15_External_20240221044618 |
| *      | 4.15    | External | vsphere  | 4.15_External_20240228043414 |
|        | 4.15    | None     | None     | 4.15_None_20240221041256     |
| *      | 4.15    | None     | None     | 4.15_None_20240228041900     |
+--------+---------+----------+----------+------------------------------+
```

- Review the summary for a latest artifact from a specific release:

```bash
$ opct adm baseline get --platform=External --version=4.15 -o /tmp/baseline-summary.json
```

- Publish many artifacts to the OPCT services (**administrative only**):

```bash
export PROCESS_FILES="4.15.0-rc.7-20240221-HighlyAvailable-vsphere-None.tar.gz
4.15.0-rc.7-20240221-HighlyAvailable-vsphere-External.tar.gz
4.15.0-rc.1-20240110-HighlyAvailable-vsphere-External.tar.gz
4.15.0-20240228-HighlyAvailable-vsphere-None.tar.gz
4.15.0-20240228-HighlyAvailable-vsphere-External.tar.gz"

# Upload each baseline artifact
for PF in $PROCESS_FILES;
do
    opct adm baseline publish --log-level=debug "$HOME/opct/s3-bucket-results/v0.4.0/default/$PF";
done

# re-index
opct adm baseline indexer

# Expire CloudFront cache if you received an error:
# - AWS Console: AWS CloudFront > Distributions > Select Distribution > Invalidations > Create new expiring '/*'
# - AWS CLI: $ aws cloudfront create-invalidation --distribution-id <id> --paths /*

# Check the latest baseline data
opct-devel adm baseline list --all

# check all baseline data
opct-devel adm baseline list
```
