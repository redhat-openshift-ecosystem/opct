# OPCT Batch Report

The Batch Report are group of reports published in S3 to compare
different executions in some activity, for example checking results
between different providers in the same OCP release.

One option is to publish the results in the blob storage, when
that option is chosed, the redirect.html may need to be added
to prevent the reports broken the relative URL references (example
when using `/reports/report-az` instead `/reports/report-az/index.html`).

To generate a batch report, you can use the script `gen-reports.sh`:

```bash
cat << EOF > ./report-4.14.txt
ocp414rc0_AWS_None_202309222127_sonobuoy_47efe9ef-06e4-48f3-a190-4e3523ff1ae0.tar.gz=ocp414rc0_AWS_None_202309222127
ocp414rc0_OCI_None_202309230246_sonobuoy_1c9b84cf-af21-461a-8aa8-30f6187b6641.tar.gz
EOF

./gen-reports.sh gen report-4.14.txt

# or to generate and upload

./gen-reports.sh upload report-4.14.txt

# to check it locally: start the file server

./gen-reports.sh serve report-4.14
```