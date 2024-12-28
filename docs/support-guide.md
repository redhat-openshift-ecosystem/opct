# OPCT - Support Guide

- [Support Case Check List](#check-list)
    - [New Support Cases](#check-list-new-case)
    - [New Executions](#check-list-new-executions)
- [Setting up the Review Environment](#setup)
    - [Install tools](#setup-install)
    - [Download dependencies](#setup-download-baseline)
    - [Download Partner Results](#setup-download-results)
- [Review guide: exploring the failed tests](#review-process)
    - [Exploring the failures](#review-process-exploring)
    - [Extracting the failures to the local directory](#review-process-extracting)
    - [Explaning the extracted files](#review-process-explain)
    - [Review Guidelines](#review-process-guidelines)
- [Review guide: y-stream upgrade](#upgrade-review-process)

## Support Case Check List <a name="check-list"></a>

### New Support Cases <a name="check-list-new-case"></a>

Check-list to require when **new** support case has been opened:

- Documentation: Installing Steps containing the flavors/size of the Infrastructure and the steps to install OCP
- Documentation: Diagram of the Architecture including zonal deployment
- Archive with Conformance results
- Archive with must-gather
- [Installation Checklist (file `user-installation-checklist.md`)](./user-installation-checklist.md) with the partner's update to sign off post-instalation items

### New Executions <a name="check-list-new-executions"></a>

The assets below, conformance assets, should be updated when certain conditions happen:

- Conformance Results
- Must Gather
- Install Documentation (when any item/flavor/configuration has been modified)


The following conditions require new conformance assets:

- The version of the OpenShift Container Platform has been updated
- Any Infrastructure component(s) (e.g.: server size, disk category, ELB type/size/config) or cluster dependencies (e.g.: external storage backend for image registry) have been modified


## Review Environment <a name="setup"></a>

### Install Tools <a name="setup-install"></a>

- Download the [opct](./user.md#install): OPCT
- Download the [`omg`](https://github.com/kxr/o-must-gather): tool to analyse Must-gather archive
```bash
pip3 install o-must-gather --user
```

### Download Baseline CI results <a name="setup-download-baseline"></a>

The OPCT run periodically ([source code](https://github.com/openshift/release/blob/master/ci-operator/jobs/redhat-openshift-ecosystem/opct/redhat-openshift-ecosystem-provider-certification-tool-main-periodics.yaml)) in OpenShift CI using the latest stable release of OpenShift.
These baseline results are stored long-term in an AWS S3 bucket (`s3://openshift-provider-certification/baseline-results`). An HTML listing can be found [here](https://openshift-provider-certification.s3.us-west-2.amazonaws.com/index.html).
These baseline results should be used as a reference when reviewing a partner's conformance results.

1. Identify cluster version in the partner's must gather:
```bash
$ omg get clusterversion
NAME     VERSION  AVAILABLE  PROGRESSING  SINCE  STATUS
version  4.11.13   True       False        11h    Cluster version is 4.11.13
```
2. Navigate to the [CI results](https://openshift-provider-certification.s3.us-west-2.amazonaws.com/index.html) and find the latest results (by date) for the matching OpenShift version
3. Download the *latest* test results for the version (bottom of list). Copy the results archive link from the webpage in previous step. 
```bash
$ curl --output 4.11.13-20221125.tar.gz https://openshift-provider-certification.s3.us-west-2.amazonaws.com/baseline-results/4.11.13-20221125.tar.gz
$ file 4.11.13-20221125.tar.gz 
4.11.13-20221125.tar.gz: gzip compressed data, original size modulo 2^32 430269440
```

### Download Partner Results <a name="setup-download-results"></a>

- Download the conformance archive from the Support Case. Example file name: `retrieved-archive.tar.gz`
- Download the Must-gather from the Support Case. Example file name: `must-gather.tar.gz`

## Review guide: exploring the failed tests <a name="review-process"></a>

The steps below use the subcommand `report` to apply filters on the failed tests and help to keep the initial focus of the investigation on the failures exclusively on the partner's results.

The filters use only tests included in the respective suite, isolating from common failures identified on the Baseline results or Flakes from CI. To see more details about the filters, read the [dev documentation describing filters flow](./dev.md#dev-diagram-filters).

Required to use this section:

- OPCT CLI downloaded to the current directory
- OpenShift e2e test suite exported to the current directory
- Baseline results exported to the current directory
- The conformance result is in the current directory


### Exploring the failures <a name="review-process-exploring"></a>

Compare the provider results with the baseline:

> `--baseline` is optional. You must use a trusted baseline results to apply the filters. Otherwise leave it unset.

```bash
./opct report \
    --baseline ./opct_baseline-ocp_4.11.4-platform_none-provider-date_uuid.tar.gz \
    ./<timestamp>_sonobuoy_<uuid>.tar.gz
```

### Extracting the failures to a local directory <a name="review-process-extracting"></a>

Compare the results and extract the files (option `--save-to`) to the local directory `./results-provider-processed`:

```bash
./opct report \
    --baseline ./opct_baseline-ocp_4.11.4-platform_none-provider-date_uuid.tar.gz \
    --save-to ./results-provider-processed \
    ./<timestamp>_sonobuoy_<uuid>.tar.gz
```

This is the expected output:

> Note: the tabulation is not ok when pasting to Markdown

```bash
(...Header...)

$ ./opct report 4.12.1-20230131.tar.gz --save-to  ./results-provider-processed
INFO[2023-02-01T01:26:25-03:00] Processing Plugin 05-openshift-cluster-upgrade... 
INFO[2023-02-01T01:26:25-03:00] Ignoring Plugin 05-openshift-cluster-upgrade 
INFO[2023-02-01T01:26:25-03:00] Processing Plugin 10-openshift-kube-conformance... 
INFO[2023-02-01T01:26:25-03:00] Processing Plugin 20-openshift-conformance-validated... 
INFO[2023-02-01T01:26:26-03:00] Processing Plugin 99-openshift-artifacts-collector... 
INFO[2023-02-01T01:26:26-03:00] Ignoring Plugin 99-openshift-artifacts-collector 
WARN[2023-02-01T01:26:27-03:00] Ignoring to populate source 'baseline'. Missing or invalid baseline artifact (-b):  

> OpenShift Provider Certification Summary <

 Kubernetes API Server version		: v1.25.4+a34b9e9
 OpenShift Container Platform version	: 4.12.1
 - Cluster Update Progressing		: False
 - Cluster Target Version		: Cluster version is 4.12.1
						
 OCP Infrastructure:			
 - PlatformType				: None
 - Name					: ci-op-nykh40v7-7280e-bsghd
 - Topology				: HighlyAvailable
 - ControlPlaneTopology			: HighlyAvailable
 - API Server URL			: https://api.ci-op-nykh40v7-7280e.vmc-ci.devcluster.openshift.com:6443
 - API Server URL (internal)		: https://api-int.ci-op-nykh40v7-7280e.vmc-ci.devcluster.openshift.com:6443
						
 Plugins summary by name:		  Status [Total/Passed/Failed/Skipped] (timeout)
 - 10-openshift-kube-conformance	: failed [691/669/22/0] (0)
 - 20-openshift-conformance-validated	: failed [3793/1627/52/2114] (0)
									
 Health summary:			  [A=True/P=True/D=True]	
 - Cluster Operators			: [33/0/0]
 - Node health				: 6/6  (100%)
 - Pods health				: 250/258  (96%)

> Processed Summary <

 Total tests by conformance suites:
 - kubernetes/conformance: 359 
 - openshift/conformance: 3454 

 Result Summary by conformance plugins:
 - 10-openshift-kube-conformance:
   - Status: failed
   - Total: 691
   - Passed: 669
   - Failed: 22
   - Timeout: 0
   - Skipped: 0
   - Failed (without filters) : 22
   - Failed (Filter SuiteOnly): 0
   - Failed (Filter CI Flakes): 0
   - Status After Filters     : pass
 - 20-openshift-conformance-validated:
   - Status: failed
   - Total: 3793
   - Passed: 1627
   - Failed: 52
   - Timeout: 0
   - Skipped: 2114
   - Failed (without filters) : 52
   - Failed (Filter SuiteOnly): 22
   - Failed (Filter CI Flakes): 3
   - Status After Filters     : failed

 Result details by conformance plugins: 


 => 10-openshift-kube-conformance: (0 failures, 0 flakes)

 --> Failed tests to Review (without flakes) - Immediate action:
<empty>

 --> Failed flake tests - Statistic from OpenShift CI
<empty>


 => 20-openshift-conformance-validated: (22 failures, 19 flakes)

 --> Failed tests to Review (without flakes) - Immediate action:
[sig-arch] Managed cluster should set requests but not limits [Suite:openshift/conformance/parallel]
[sig-cli] oc basics can get version information from API [Suite:openshift/conformance/parallel]
[sig-scheduling] SchedulerPriorities [Serial] PodTopologySpread Scoring validates pod should be preferably scheduled to node which makes the matching pods more evenly distributed [Suite:openshift/conformance/serial] [Suite:k8s]

 --> Failed flake tests - Statistic from OpenShift CI
Flakes	Perc		 TestName
1	0.134%		[sig-api-machinery][Feature:APIServer] anonymous browsers should get a 403 from / [Suite:openshift/conformance/parallel]
1	0.134%		[sig-arch] Managed cluster should ensure control plane pods do not run in best-effort QoS [Suite:openshift/conformance/parallel]
748	100.000%	[sig-arch] Managed cluster should ensure platform components have system-* priority class associated [Suite:openshift/conformance/parallel]
--	--		[sig-arch][Late] clients should not use APIs that are removed in upcoming releases [apigroup:config.openshift.io] [Suite:openshift/conformance/parallel]
(...)

 Data Saved to directory './results-provider-processed/'

```


### Understanding the extracted results <a name="review-process-explain"></a>

The data extracted to local storage contains the following files for each plugin:

- `test_${PLUGIN_NAME}_baseline_failures.txt`: List of test failures from the baseline execution
- `test_${PLUGIN_NAME}_provider_failures.txt`: List of test failures from the execution
- `test_${PLUGIN_NAME}_provider_filter1-suite.txt`: List of test failures included on suite
- `test_${PLUGIN_NAME}_provider_filter2-baseline.txt`: List of test failures tests* after applying all filters
- `test_${PLUGIN_NAME}_provider_suite_full.txt`: List with suite e2e tests

The base directory (`./results-provider-processed`) also contains the **all error messages (stdout and fail summary)** for each failed test. Those errors are saved into individual files onto those sub-directories (for each plugin):

- `failures-baseline/${PLUGIN_NAME}_${INDEX}-failure.txt`: the error summary
- `failures-baseline/${PLUGIN_NAME}_${INDEX}-systemOut.txt`: the entire stdout of the failed plugin

Considerations:

- `${PLUGIN_NAME}`: currently these plugins names are valid: [`openshift-validated`, `kubernetes-conformance`]
- `${INDEX}` is the simple index ordered by test name on the list

Example of files on the extracted directory:

```bash
$ tree ./results-provider-processed
processed/
├── failures-baseline
[redacted]
├── failures-provider
[redacted]
├── failures-provider-filtered
│   ├── kubernetes-conformance_1-1-failure.txt
│   ├── kubernetes-conformance_1-1-systemOut.txt
│   ├── kubernetes-conformance_2-2-failure.txt
│   ├── kubernetes-conformance_2-2-systemOut.txt
│   ├── openshift-validated_1-31-failure.txt
│   ├── openshift-validated_1-31-systemOut.txt
[redacted]
│   ├── openshift-validated_7-1-failure.txt
│   └── openshift-validated_7-1-systemOut.txt
├── tests_kubernetes-conformance_baseline_failures.txt
├── tests_kubernetes-conformance_provider_failures.txt
├── tests_kubernetes-conformance_provider_filter1-suite.txt
├── tests_kubernetes-conformance_provider_filter2-baseline.txt
├── tests_kubernetes-conformance_suite_full.txt
├── tests_openshift-validated_baseline_failures.txt
├── tests_openshift-validated_provider_failures.txt
├── tests_openshift-validated_provider_filter1-suite.txt
├── tests_openshift-validated_provider_filter2-baseline.txt
└── tests_openshift-validated_suite_full.txt

3 directories, 300 files
```

### Review Guidelines <a name="review-process-guidelines"></a>

> WIP: the idea here is to provide guidance on the main points/assets to review, pointing to the details on the respective/dedicated sections.

This section is a guide of the initial files to review when start exploring the resulting archive.

Items to review:

- OCP version matches the ticket request
- Review the result file
- Check if the failures are 0, if not, need to check one by one
- To provide a better interaction between the review process, one spreadsheet named `failures-index.xlsx` is created inside the extracted directory (`./processed/` exemplified in the last section). It can be used as a tool to review failures and take notes about them.
- Check details of each test failed on the sub-directory `failures-provider-filtered/*.txt`.

Additional items to review:

- explore the must-gather objects according to findings on the failures files
- run insights rules on the must-gather to check if there's a new know issue: `insights run -p ccx_rules_ocp ${MUST_GATHER_PATH}`
> TODO: provide steps to install and run insight OCP rules (opct could provide one container with it installed to avoid overhead and environment issues)

## Review Guide: Manual Y-Stream Upgrade <a name="upgrade-review-process"></a>

The validation process (when applyging for the Partner Support Case) requires a successful y-stream upgrade (e.g. upgrade 4.11.17 to 4.12.0).
Upgrade review should only proceed if there is reasonably high confidence in passing and not if there are still significant issues in passing the review process above. 

> TODO: Review this documentation after the automated upgrade procedure is merged in https://github.com/redhat-openshift-ecosystem/opct/pull/33

Once prepared to review an upgrade, this is the recommended procedure:

1. Cloud provider to install _new_ cluster as the version previously reviewed in the process above
2. Initiate upgrade to next Y-stream version per OpenShift documentation 
3. Cloud provider to make note of the following during upgrade:

   - Any manual intervention required during upgrade
   - Time taken to complete upgrade 
   - Any components left in failed state or not upgraded (e.g. web console offline, inaccessible API)

4. Must gather after successful or failed upgrade

If there was manual intervention required during the upgrade this will require judgement of the OpenShift engineer reviewing the upgrade. 
Some questions to ask are:

Is the manual intervention...

- Working around a known bug in OpenShift?
- Working around a potential new bug in OpenShift?
- Working around a known issue in OpenShift but not considered a bug and has documentation?
- Working around an issue specific to the cloud provider?

If the answer to any of the questions above is "Yes" then take the necessary steps to remediate the situation (if needed) through 
documentation, bug reports, and escalations to meet the Validation requirements.

After a successful upgrade where any manual interventions aren't a blocker, review the Must Gather that was captured. 
First, check the `ClusterVersion` resource to verify the upgrade was successful:

Using the `omg` tool:

```bash
omg get clusterversion
```

Next, check each Cluster Operator and Node was upgraded and in a working/ready state:

```bash
omg get clusteroperators
omg get nodes
```

Review the Must Gather using the Insights tool as mentioned [here](#review-process-guidelines).

If there are any issues found in the steps above, the upgrade should be performed again (on a _new_ cluster) and upgrade review process restarted.
