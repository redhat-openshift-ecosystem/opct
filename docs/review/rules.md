# OPCT Review/Check Rules

The OPCT rules are used in the `report` command to evaluate the data collected by
the OPCT execution. The HTML report will link directly to the rule ID on this page.

The rule details can be used as an additional resource in the review process.

The acceptance criteria for the rules are based on the CI results.

## Rules
___
### OPCT-001 <a name="OPCT-001"></a>

- **Name**: Plugin Conformance Kubernetes [10-openshift-kube-conformance] must pass (after filters)
- **Description**: Kubernetes Conformance suite (defined as `kubernetes/conformance` in `openshift-tests`) implements e2e required by Kubernetes Certification.

Expected:
```
 - 10-openshift-kube-conformance:
...
   - Failed (Filter SuiteOnly): 0 (0.00%)
   - Failed (Priority)        : 0 (0.00%)
   - Status After Filters     : passed
```

- **Troubleshooting**:

Review the High-Priority Failures:
```sh
$ /opct-dev report archive.tar.gz
(..)
 => 10-openshift-kube-conformance: (2 failures, 0 flakes)

 --> Failed tests to Review (without flakes) - Immediate action:
[total=2] [sig-apps=1 (50.00%)] [sig-api-machinery=1 (50.00%)]

15	[sig-apps] Deployment deployment should support proportional scaling [Conformance] [Suite:openshift/conformance/parallel/minimal] [Suite:k8s]
6	[sig-api-machinery] Aggregator Should be able to support the 1.17 Sample API Server using the current Aggregator [Conformance] [Suite:openshift/conformance/parallel/minimal] [Suite:k8s]

 --> Failed flake tests - Statistic from OpenShift CI
[total=0]

Flakes	Perc	ErrCount	 TestName

```
___
### OPCT-002 <a name="OPCT-002"></a>

- **Name**: Plugin Conformance Upgrade [05-openshift-cluster-upgrade] must pass
- **Description**: The upgrade conformance suite runs e2e tests while running upgrade using `openshift-tests` tool. The overall result must be passed.
___
### OPCT-003 <a name="OPCT-003"></a>

- **Name**: Plugin Collector [99-openshift-artifacts-collector] must pass.
- **Description**: The Collector plugin is responsible to retrieve information from the cluster, including must-gather, etcd parsed logs, e2e test lists for conformance suites. It is expected the value of `passed` in the state, otherwise, the review flow will be impacted.
- **Troubleshooting**:

Check the failed tests:
```sh
$ ./opct results -p 99-openshift-artifacts-collector archive.tar.gz
```

Check the plugin logs:
```sh
$ grep -B 5 'Creating failed JUnit' \
    podlogs/openshift-provider-certification/sonobuoy-99-*/logs/plugin.txt
```
___
### OPCT-004 <a name="OPCT-004"></a>

- **Name**: OpenShift Conformance [20-openshift-conformance-validated]: Failed tests must report less than 1.5%
- **Description**: OpenShift Conformance suite must not report a high number of failures in the base execution. Ideally, the lower is better, but the e2e tests are frequently being updated/improved fixing bugs and eventually, the tested release could be impacted by those issues. The reference of 1.5% baseline is from executions in known platforms. Higher failures could be related to errors in the tested environment. Check the test logs to isolate the issues.
- **Action**: Check the failures section `Test failures [high priority]`

___
### OPCT-005 <a name="OPCT-005"></a>
- **Name**: OpenShift Conformance [20-openshift-conformance-validated]: Priority must report less than 0.5%
- **Description**: OpenShift Conformance suite must not report a high number of failures after applying filters. Ideally, the lower is better, but the e2e tests are frequently being updated/improved fixing bugs and eventually, the tested release could be impacted by those issues. The reference of 0.5% baseline is from executions in known platforms. Higher failures could be related to errors in the tested environment. Check the test logs to isolate the issues.
- **Action**: Check the failures section `Test failures [high priority]`

___
### OPCT-006 <a name="OPCT-006"></a>
- **Name**: Suite Errors must report a lower number of log errors
- **Description**: The Conformance suites are reporting a high number of errors.
- **Action**: Check test logs to isolate the errors.
- **Troubleshooting**:

To check the error counter by e2e test using HTML report navigate to `Suite Errors` in the left menu and table `Tests by Error Pattern`.

To check the logs, navigate to the Plugin menu and check the logs `failure` and `systemOut`.

___
### OPCT-007 <a name="OPCT-007"></a>
- **Name**: Workloads must report a lower number of errors in the logs
- **Description**: Workloads collected are reporting a high number of errors.
- **Action**: Check pod logs to isolate the issue.
- **Troubleshooting**:

To check the error counter by test using HTML report navigate to `Workload Errors` in the left menu. The table `Error Counters by Namespace` shows the namespace reporting a high number of errors, rank by the higher, you can start exploring the logs in that namespace.

The table `Error Counters by Pod and Pattern` in `Workload Errors` menu also report the pods
you also can use that information to isolate any issue in your environment.

To explore the logs, you can extract the must-gather collected by plugin `99-openshift-artifacts-collector`:

```sh
# extract must-gather from the results
tar xfz artifact.tar.gz \
    plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather.tar.xz

# extract must-gather
mkdir must-gather && \
    tar xfJ plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather.tar.xz \
    -C must-gather

# check workload logs with omc (example etcd)
omc use must-gather
omc logs -n openshift-etcd etcd-control-plane-0 -c etcd
```
___
### OPCT-008 <a name="OPCT-008"></a>
- **Name**: All nodes must be healthy
- **Description**: All nodes in the cluster must be ready.
- **Action**: Check the nodes and the reason it is not reporting as ready.
- **Troubleshooting**:

Check the unhealthy nodes in the cluster:
```sh
$ omc get nodes
```

Review the node and events:
```sh
$ omc describe node <node_name>
```

___
### OPCT-009 <a name="OPCT-009"></a>
- **Name**: Pods Healthy must report be higher than 98%
- **Description**: Pods must report healthy.
- **Action**: Check the failing pod, isolate if it is related with the environment and/or the validation tests.
- **Troubleshooting**:

Check the unhealthy pods:
```sh
$ ./opct report archive.tar.gz
(...)
 Health summary:			  [A=True/P=True/D=True]	
 - Cluster Operators			: [33/0/0]
 - Node health				: 6/6  (100.00%)
 - Pods health				: 246/247  (99.00%)
						
 Failed pods:
  Namespace/PodName						Healthy	Ready	Reason		Message
  openshift-kube-controller-manager/installer-6-control-plane-1	false	False	PodFailed	
(...)
```

Explore the pods:
```sh
$ omc get pods -A |egrep -v '(Running|Completed)'
```
___
<!-- 
> Add new tests after "___" using the following template.
___
### OPCT-000 <a name="OPCT-000"></a>

**Name**: Rule Name

**Description**: Plugin description

**Actions**:

- action 1

-->
___
### OPCT-007 <a name="OPCT-007"></a>
- **Name**: Workloads must report a lower number of errors in the logs
- **Description**: Workloads collected are reporting a high number of errors.
- **Action**: Check pod logs to isolate the issue.
- **Troubleshooting**:

To check the error counter by e2e test using HTML report navigate to `Workload Errors` in the left menu. The table `Error Counters by Namespace` shows the namespace reporting a high number of errors, rank by the higher, you can start exploring the logs in that namespace.

The table `Error Counters by Pod and Pattern` in `Workload Errors` menu also report the pods
you also can use that information to isolate any issue in your environment.

To explore the logs, you can extract the must-gather collected by the plugin `99-openshift-artifacts-collector`:

```sh
# extract must-gather from the results
tar xfz artifact.tar.gz \
    plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather.tar.xz

# extract must-gather
mkdir must-gather && \
    tar xfJ plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather.tar.xz \
    -C must-gather

# check workload logs with `omc` (example etcd)
omc use must-gather
omc logs -n openshift-etcd etcd-control-plane-0 -c etcd
```
___
### OPCT-008 <a name="OPCT-008"></a>
- **Name**: All nodes must be healthy
- **Description**: All nodes in the cluster must be ready.
- **Action**: Check the nodes and the reason it is not reporting as ready.
- **Troubleshooting**:

Check the unhealthy nodes in the cluster:
```sh
$ omc get nodes
```

Review the node and events:
```sh
$ omc describe node <node_name>
```

___
### OPCT-009 <a name="OPCT-009"></a>
- **Name**: Pods Healthy must report higher than 98%
- **Description**: Pods must report healthy.
- **Action**: Check the failing pod, and isolate if it is related to the environment and/or the validation tests.
- **Troubleshooting**:

Check the unhealthy pods:
```sh
$ ./opct report archive.tar.gz
(...)
 Health summary:              [A=True/P=True/D=True]    
 - Cluster Operators            : [33/0/0]
 - Node health              : 6/6  (100.00%)
 - Pods health              : 246/247  (99.00%)
                        
 Failed pods:
  Namespace/PodName                     Healthy Ready   Reason      Message
  openshift-kube-controller-manager/installer-6-control-plane-1 false   False   PodFailed   
(...)
```

Explore the pods:
```sh
$ omc get pods -A |egrep -v '(Running|Completed)'
```
___
<!-- 
> Add new tests after "___" using the following template.
___
### OPCT-000 <a name="OPCT-000"></a>

**Name**: Rule Name

**Description**: Plugin description

**Actions**:

- action 1

-->