// Description: This file contains the implementation of the SLO interface,
// translated to "checks" in the OPCT report package. The SLO interface is defined
// in the report package, and the package implements SLIs to ensure acceptance
// criteria is met in the data collected from artifacts.
// Reference: https://github.com/kubernetes/community/blob/master/sig-scalability/slos/slos.md
package report

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/plugin"
	log "github.com/sirupsen/logrus"
)

const (
	docsRulesPath  = "/review/rules"
	defaultBaseURL = "https://redhat-openshift-ecosystem.github.io/opct"

	CheckResultNamePass CheckResultName = "pass"
	CheckResultNameFail CheckResultName = "fail"
	CheckResultNameWarn CheckResultName = "warn"
	CheckResultNameSkip CheckResultName = "skip"

	CheckIdEmptyValue string = "--"

	// SLOs
	CheckID001  string = "OPCT-001"
	CheckID004  string = "OPCT-004"
	CheckID005  string = "OPCT-005"
	CheckID022  string = "OPCT-022"
	CheckID023A string = "OPCT-023A"
	CheckID023B string = "OPCT-023B"
)

type CheckResultName string

type CheckResult struct {
	Name    CheckResultName `json:"result"`
	Message string          `json:"message"`
	Target  string          `json:"want"`
	Actual  string          `json:"got"`
}

func (cr *CheckResult) String() string {
	return string(cr.Name)
}

type SLOOutput struct {
	ID  string `json:"id"`
	SLO string `json:"slo"`

	// SLOResult is the target value
	SLOResult string `json:"sloResult"`

	// SLITarget is the target value
	SLITarget string `json:"sliTarget"`

	// SLICurrent is the indicator result. Allowed values: pass|fail|skip
	SLIActual string `json:"sliCurrent"`

	Message string `json:"message"`

	Documentation string `json:"documentation"`
}

type Check struct {
	// ID is the unique identifier for the check. It is used
	// to mount the documentation for each check.
	ID string `json:"id"`

	// Name is the unique name for the check to be reported.
	// It must have short and descriptive name identifying the
	// failure item.
	Name string `json:"name"`

	// Description describes shortly the check.
	Description string `json:"description"`

	// Documentation must point to documentation URL to review the
	// item.
	Documentation string `json:"documentation"`

	// DocumentationSpec is the detailed documentation for the check.
	DocumentationSpec CheckDocumentationSpec `json:"documentationSpec"`

	// Accepted must report acceptance criteria, when true
	// the Check is accepted by the tool, otherwise it is
	// failed and must be reviewede.
	Result CheckResult `json:"result"`

	// ResultMessage string `json:"resultMessage"`

	Test func() CheckResult `json:"-"`

	// Priority is the priority to execute the check.
	// 0 is higher.
	Priority uint64
}

// CheckDocumentationSpec is the detailed documentation for the check.
type CheckDocumentationSpec struct {
	Description  string   `json:"description"`
	Expected     string   `json:"expected"`
	Troubleshoot string   `json:"troubleshoot"`
	Action       string   `json:"action"`
	Dependencies []string `json:"dependencies"`
}

func ExampleAcceptanceCheckPass() CheckResultName {
	return CheckResultNamePass
}

func AcceptanceCheckFail() CheckResultName {
	return CheckResultNameFail
}

// func CheckRespCustomFail(custom string) CheckResult {
// 	resp := CheckResult(fmt.Sprintf("%s [%s]", CheckResultNameFail, custom))
// 	return resp
// }

// CheckSummary aggregates the checks.
type CheckSummary struct {
	baseURL string
	Checks  []*Check `json:"checks"`
}

func NewCheckSummary(re *ReportData) *CheckSummary {
	baseURL := defaultBaseURL
	msgDefaultNotMatch := "default value does not match the acceptance criteria"
	// Developer environment:
	// $ mkdocs serve
	// $ export OPCT_DEV_BASE_URL_DOC="http://127.0.0.1:8000/provider-certification-tool"
	localDevBaseURL := os.Getenv("OPCT_DEV_BASE_URL_DOC")
	if localDevBaseURL != "" {
		baseURL = localDevBaseURL
	}
	checkSum := &CheckSummary{
		Checks:  []*Check{},
		baseURL: fmt.Sprintf("%s%s", baseURL, docsRulesPath),
	}
	// Cluster Checks
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-020",
		Name: "All nodes must be healthy",
		Test: func() CheckResult {
			res := CheckResult{Name: CheckResultNameFail, Target: "100%"}
			if re.Provider == nil || re.Provider.ClusterHealth == nil {
				log.Debugf("Check Failed: OPCT-008: unavailable results")
				return res
			}
			res.Actual = fmt.Sprintf("%.3f%%", re.Provider.ClusterHealth.NodeHealthPerc)
			if re.Provider.ClusterHealth.NodeHealthPerc != 100 {
				log.Debugf("Check Failed: OPCT-008: want[!=100] got[%f]", re.Provider.ClusterHealth.NodeHealthPerc)
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: "All nodes must be healthy. The node health is a metric that helps to understand the health of the cluster.",
			Action:      "Check the node health section in the report and review the logs for each node.",
			Expected:    "All nodes must be healthy.",
			Troubleshoot: `One or more nodes have been detected as unhealth when the aggregator server collected the cluster state (end of job).
Unhealth nodes can cause test failures. This check can be used as a helper while investigating test failures. This check can be skipped
if it is not causing failures in the conformance tests.
Check the unhealthy nodes in the cluster:
~~~sh
$ omc get nodes
~~~
Review the node and events:
~~~sh
$ omc describe node <node_name>
~~~
`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-021",
		Name: "Pods Healthy must report higher than 98%",
		Test: func() CheckResult {
			res := CheckResult{Name: CheckResultNameFail, Target: ">=98%"}
			if re.Provider == nil || re.Provider.ClusterHealth == nil {
				return res
			}
			res.Actual = fmt.Sprintf("%.3f", re.Provider.ClusterHealth.PodHealthPerc)
			if re.Provider.ClusterHealth.PodHealthPerc < 98.0 {
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: `Pods Healthy must report higher than 98%. The pod health is a metric that helps to understand the health of some components.
			High pod health is a good indicator of the cluster health. The error budget of 2% is a reference used as a baseline from several executions in known platforms.`,
			Action:   "Check the failing pod, and isolate if it is related to the environment and/or the validation tests.",
			Expected: "Pods Healthy must report higher than 98%.",
			Troubleshoot: `One or more pods have been detected as unhealth when the aggregator server collected the cluster state (end of job).
Run the CLI command <code>opct results archive.tar.gz</code> to review the failed pods.
Explore the logs for each pods in must-gather available in the collector plugin.
Check the unhealthy pods:
~~~sh
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
~~~
Explore the pods:
~~~sh
$ omc get pods -A |egrep -v '(Running|Completed)'
~~~
`,
		},
	})
	// Plugins Checks
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   CheckID001,
		Name: "Kubernetes Conformance [10-openshift-kube-conformance] must pass 100%",
		Test: func() CheckResult {
			res := CheckResult{Name: CheckResultNameFail, Target: "Priority==0|Total!=Failed"}
			prefix := "Check Failed - " + CheckID001
			if _, ok := re.Provider.Plugins[plugin.PluginNameKubernetesConformance]; !ok {
				log.Debugf("%s Runtime: processed plugin data not found: %v", prefix, re.Provider.Plugins[plugin.PluginNameKubernetesConformance])
				return res
			}
			p := re.Provider.Plugins[plugin.PluginNameKubernetesConformance]
			if p.Stat.Total == p.Stat.Failed {
				res.Message = "Potential Runtime Failure. Check the Plugin logs."
				res.Actual = "Total==Failed"
				log.Debugf("%s Runtime: Total and Failed counters are equals indicating execution failure", prefix)
				return res
			}
			res.Actual = fmt.Sprintf("Priority==%d", len(p.FailedFiltered))
			if len(p.FailedFiltered) > 0 {
				log.Debugf("%s Acceptance criteria: FailedFiltered counter is greater than 0: %v", prefix, len(p.FailedFiltered))
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: "Kubernetes Conformance suite (defined as `kubernetes/conformance` in `openshift-tests`) implements e2e required by Kubernetes Certification. Those tests are base tests for an operational Kubernetes cluster. All tests must be passed prior reviewing OpenShift Conformance suite.",
			Action:      "Review the logs for each failed test in the Kubernetes conformance suite.",
			Expected: `~~~
 - 10-openshift-kube-conformance:
[...]
   - Failed (Filter SuiteOnly): 0 (0.00%)
   - Failed (Priority)        : 0 (0.00%)
   - Status After Filters     : passed
~~~`,
			Troubleshoot: `Review the High-Priority Failures:
~~~sh
$ /opct report archive.tar.gz
(..)
 => 10-openshift-kube-conformance: (2 failures, 0 flakes)

 --> Failed tests to Review (without flakes) - Immediate action:
[total=2] [sig-apps=1 (50.00%)] [sig-api-machinery=1 (50.00%)]

15	[sig-apps] Deployment deployment should support proportional scaling [Conformance] [Suite:openshift/conformance/parallel/minimal] [Suite:k8s]
6	[sig-api-machinery] Aggregator Should be able to support the 1.17 Sample API Server using the current Aggregator [Conformance] [Suite:openshift/conformance/parallel/minimal] [Suite:k8s]

~~~`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   CheckID004,
		Name: "OpenShift Conformance [20-openshift-conformance-validated]: Pass ratio must be >=98.5%",
		Test: func() CheckResult {
			prefix := "Check Failed - " + CheckID004
			res := CheckResult{
				Name:   CheckResultNameFail,
				Target: "Pass>=98.5%(Fail>1.5%)",
			}
			if _, ok := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]; !ok {
				return res
			}
			// "Acceptance" are relative, the baselines is observed to set
			// an "accepted" value considering a healthy cluster in known provider/installation.
			p := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]
			if p.Stat == nil {
				log.Debugf("%s Runtime: Stat not found", prefix)
				return res
			}
			if p.Stat.Total == p.Stat.Failed {
				res.Message = "Potential Runtime Failure. Check the Plugin logs."
				res.Actual = "Total==Failed"
				log.Debugf("%s Runtime: Total and Failed counters are equals indicating execution failure", prefix)
				return res
			}
			perc := (float64(p.Stat.Failed) / float64(p.Stat.Total)) * 100
			res.Actual = fmt.Sprintf("Fail==%.2f%%(%d)", perc, p.Stat.Failed)
			if perc > 1.5 {
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: `OpenShift Conformance suite must not report a high number of failures in the base execution.
Ideally, the lower is better, but the e2e tests are frequently being updated/improved fixing bugs and eventually,
the tested release could be impacted by those issues. The reference of 1.5% error budged is a reference used as basedline
from several executions in known platforms.
Higher failure ratio could be related to errors in the tested environment, cluster configuration, and/or infrastructure issues.
Check the test logs to isolate the issues.
When applying to cluster validation with Red Hat teams, this check must be reviewed immediately before submitting the results as
it is a potential problem in the infrastructure or missconfiguration.
Review the [OpenShift documentation for installing in agnostic platforms](https://docs.openshift.com/container-platform/latest/installing/installing_platform_agnostic/installing-platform-agnostic.html)`,
			Expected: `Error budget lower than 1.5% of failed tests.`,
			Troubleshoot: `
1. Load the html report and navigate to the failures
1.A. Generate the html report
~~~sh
$ /opct report --save-to ./results archive.tar.gz
$ firefox http://localhost:8000
~~~
1.B. Review the logs for each failed test`,
			Action: `Check the failures section <code>Test failures [high priority]</code> and review the logs for each failed test.`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   CheckID005,
		Name: "OpenShift Conformance Validation [20]: Filter Priority Requirement >= 99.5%",
		Test: func() CheckResult {
			prefix := "Check Failed - " + CheckID005
			target := 0.5
			res := CheckResult{
				Name:   CheckResultNameFail,
				Target: fmt.Sprintf("W<=%.2f%%,F>%.2f%%", target, target),
				Actual: "N/A",
			}
			if _, ok := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]; !ok {
				return res
			}
			// "Acceptance" are relative, the baselines is observed to set
			// an "accepted" value considering a healthy cluster in known provider/installation.
			// plugin := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]
			p := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]
			if p.Stat.Total == p.Stat.Failed {
				res.Message = "Potential Runtime Failure. Check the Plugin logs."
				res.Actual = "Total==Failed"
				log.Debugf("%s Runtime: Total and Failed counters are equals indicating execution failure", prefix)
				return res
			}
			perc := (float64(p.Stat.FilterFailedPrio) / float64(p.Stat.Total)) * 100
			res.Actual = fmt.Sprintf("Fail==%.2f%%(%d)", perc, p.Stat.FilterFailedPrio)
			if perc > target {
				res.Name = CheckResultNameFail
				return res
			}
			// if perc > 0 && perc <= target {
			// 	res.Name = CheckResultNameWarn
			// 	return res
			// }
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: `OpenShift Conformance suite must not report a high number of failures after applying filters.
Ideally, the lower is better, but the e2e tests are frequently being updated/improved fixing bugs and eventually,
the tested release could be impacted by those issues. The error budget higher than 0.5% could indicate issues in the
tested environment. Higher failures could be related to errors in the tested environment.
Check the test logs for OpenShift conformance suite, Priority section, to isolate the issues.`,
			Action: `

	1. check the failures section <code>Test failures [high priority]</code>
	2. review the logs for each failed test.
	3. the remainging failures must be reviewed individually to achieve a successfull installation. Root cause of individual failures must be identified.
`,
			Expected: `Error budget under acceptance criteria. Errors in the budget must be reviewed and root cause identified.`,
			//Troubleshoot: ``,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-005B",
		Name: "OpenShift Conformance Validation [20]: Required to Pass After Filters",
		Test: func() CheckResult {
			prefix := "Check OPCT-005B Failed"
			target := 0.50
			res := CheckResult{
				Name:   CheckResultNameFail,
				Target: fmt.Sprintf("Pass==100%%(W<=%.2f%%,F>%.2f%%)", target, target),
				Actual: "N/A",
			}
			if _, ok := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]; !ok {
				return res
			}
			// "Acceptance" are relative, the baselines is observed to set
			// an "accepted" value considering a healthy cluster in known provider/installation.
			// plugin := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]
			p := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]
			if p.Stat.Total == p.Stat.Failed {
				res.Message = "Potential Runtime Failure. Check the Plugin logs."
				res.Actual = "Total==Failed"
				log.Debugf("%s Runtime: Total and Failed counters are equals indicating execution failure", prefix)
				return res
			}
			perc := (float64(p.Stat.FilterFailures) / float64(p.Stat.Total)) * 100
			res.Actual = fmt.Sprintf("Fail==%.2f%%(%d)", perc, p.Stat.FilterFailures)
			if perc > target {
				res.Name = CheckResultNameFail
				return res
			}
			if perc > 0 && perc <= target {
				res.Name = CheckResultNameWarn
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description:  `OpenShift Conformance suite must report passing after applying filters removing common/well-known issues.`,
			Action:       "Check the failures section `Test failures [high priority]`. Dependencies must be passing prior this check.",
			Dependencies: []string{"OPCT-004", "OPCT-005"},
		},
	})

	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-011",
		Name: "The test suite generates accepted error budget",
		Test: func() CheckResult {
			// threshold for warn and fail
			thWarn := 150
			thFail := 300
			res := CheckResult{
				Name:   CheckResultNameWarn,
				Target: fmt.Sprintf("Pass<=%d(W>%d,F>%d)", thWarn, thWarn, thFail),
				Actual: "N/A",
			}
			if re.Provider.ErrorCounters == nil {
				res.Name = CheckResultNameWarn
				res.Actual = "No counters"
				return res
			}
			cnt := *re.Provider.ErrorCounters
			if _, ok := cnt["total"]; !ok {
				res.Message = "Unable to load Total Counter"
				res.Name = CheckResultNameFail
				res.Actual = "ERR !total"
				return res
			}
			// "Acceptance" are relative, the baselines is observed to set
			// an "accepted" value considering a healthy cluster in known provider/installation.
			total := cnt["total"]
			res.Actual = fmt.Sprintf("%d", total)
			// Error
			if total > thFail {
				res.Name = CheckResultNameFail
				return res
			}
			// Warn
			if total > thWarn {
				return res
			}
			// 0? really? something went wrong!
			if total == 0 {
				res.Name = CheckResultNameFail
				res.Actual = "WARN missing counters"
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: `The test suite generates accepted error budget. The error budget is a metric that helps to understand the
health of the test suite. The error budget is the total number of errors that the test suite can generate before it is considered
unreliable. The error budget is a relative value and it is based on the observed values in known platforms.
To check the error counter by e2e test using HTML report navigate to <code>Suite Errors</code> in the left menu and table <code>Tests by Error Pattern</code>.
To check the logs, navigate to the Plugin menu and check the logs <code>failure</code> and <code>systemOut</code>.
`,
			Action:       "Check the errors section in the report and resolve the log failures for the failed test.",
			Expected:     "The error budget is a relative value and it is based on the observed values in known platforms.",
			Troubleshoot: "Open the error budget section in the report and review the logs for each failed test.",
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-010",
		Name: "The cluster logs generates accepted error budget",
		Test: func() CheckResult {
			passLimit := 30000
			failLimit := 100000
			res := CheckResult{
				Name:   CheckResultNameFail,
				Target: "W:<=30k,F:>100k",
				Actual: "N/A",
			}
			prefix := "Check OPCT-007 Failed"
			if re.Provider.MustGatherInfo == nil {
				log.Debugf("%s: MustGatherInfo is not defined", prefix)
				res.Name = CheckResultNameFail
				res.Actual = "ERR !must-gather"
				return res
			}
			if _, ok := re.Provider.MustGatherInfo.ErrorCounters["total"]; !ok {
				log.Debugf("%s: OPCT-007: ErrorCounters[\"total\"]", prefix)
				res.Name = CheckResultNameFail
				res.Actual = "ERR !counters"
				return res
			}
			// "Acceptance" are relative, the baselines is observed to set
			// an "accepted" value considering a healthy cluster in known provider/installation.
			total := re.Provider.MustGatherInfo.ErrorCounters["total"]
			res.Actual = fmt.Sprintf("%d", total)
			if total > passLimit && total < failLimit {
				res.Name = CheckResultNameWarn
				log.Debugf("%s WARN acceptance criteria: want[<=%d] got[%d]", prefix, passLimit, total)
				return res
			}
			if total >= failLimit {
				res.Name = CheckResultNameFail
				log.Debugf("%s FAIL acceptance criteria: want[<=%d] got[%d]", prefix, passLimit, total)
				return res
			}
			// 0? really? something went wrong!
			if total == 0 {
				log.Debugf("%s FAIL acceptance criteria: want[!=0] got[%d]", prefix, total)
				res.Name = CheckResultNameFail
				res.Actual = "ERR total==0"
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: `The cluster logs, must-gather event logs, should generate fewer error in the logs. The error budget are a metric that helps to isolate the
health of the cluster. The error counters are a relative value and it is based on the observed values in CI executions in tested providers/platforms.`,
			Action:   "Check the errors section in the report, explore the logs for each service in must-gather - using tools like omc, omg, grep, etc (must-gather readers/explorer).",
			Expected: "The error events in must-gather are a relative value and it is based on the observed values in known platforms.",
			Troubleshoot: `Open the error events section in the report and review the rank of failed keywords, then check the rank by namespace and services for each failure.
Error budgets helps to focus in specific services that may contribute to the cluster failures.

To check the error counter by e2e test using HTML report navigate to <code>Workload Errors</code> in the left menu.
The table <code>Error Counters by Namespace</code> shows the namespace reporting a high number of errors, rank by the higher,
you can start exploring the logs in that namespace.

The table <code>Error Counters by Pod and Pattern</code> in <code>Workload Errors</code> menu also report the pods
you also can use that information to isolate any issue in your environment.

To explore the logs, you can extract the must-gather collected by the plugin <code>99-openshift-artifacts-collector</code>:

~~~sh
# extract must-gather from the results
tar xfz artifact.tar.gz \
    plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather.tar.xz

# extract must-gather
mkdir must-gather && \
tar xfJ plugins/99-openshift-artifacts-collector/results/global/artifacts_must-gather.tar.xz \
-C must-gather

# check workload logs with 'omc' (example etcd)
omc use must-gather
omc logs -n openshift-etcd etcd-control-plane-0 -c etcd
~~~
`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-003",
		Name: "Plugin Collector [99-openshift-artifacts-collector] must pass",
		Test: func() CheckResult {
			prefix := "Check OPCT-003 Failed"
			res := CheckResult{Name: CheckResultNameFail, Target: "passed", Actual: "N/A"}
			if _, ok := re.Provider.Plugins[plugin.PluginNameArtifactsCollector]; !ok {
				return res
			}
			p := re.Provider.Plugins[plugin.PluginNameArtifactsCollector]
			if p.Stat.Total == p.Stat.Failed {
				log.Debugf("%s Runtime: Total and Failed counters are equals indicating execution failure", prefix)
				return res
			}
			// Acceptance check
			res.Actual = re.Provider.Plugins[plugin.PluginNameArtifactsCollector].Stat.Status
			if res.Actual == "passed" {
				res.Name = CheckResultNamePass
				return res
			}
			log.Debugf("%s: %s", prefix, msgDefaultNotMatch)
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: "The Collector plugin is responsible to retrieve information from the cluster, including must-gather, etcd parsed logs, e2e test lists for conformance suites. It is expected the value of `passed` in the state, otherwise, the review flow will be impacted.",
			Expected:    "The artifacts collector plugin must pass the execution.",
			Troubleshoot: `Review the artifacts collector logs and check the artifacts generated by the plugin:
- Check the failed tests:
~~~sh
$ ./opct results -p 99-openshift-artifacts-collector archive.tar.gz
~~~

- Check the plugin logs:
~~~sh
$ grep -B 5 'Creating failed JUnit' \
	podlogs/openshift-provider-certification/sonobuoy-99-*/logs/plugin.txt
~~~
`,
			Action: "Check the artifacts collector logs (click under the test name in the job list). If the cause is undefined, re-run the execution.",
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-002",
		Name: "Plugin Conformance Upgrade [05-openshift-cluster-upgrade] must pass",
		Test: func() CheckResult {
			prefix := "Check OPCT-002 Failed"
			res := CheckResult{Name: CheckResultNameFail, Target: "passed"}
			if _, ok := re.Provider.Plugins[plugin.PluginNameOpenShiftUpgrade]; !ok {
				return res
			}
			res.Actual = re.Provider.Plugins[plugin.PluginNameOpenShiftUpgrade].Stat.Status
			if res.Actual == "passed" {
				res.Name = CheckResultNamePass
				return res
			}
			log.Debugf("%s: %s", prefix, msgDefaultNotMatch)
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description:  "The cluster upgrade plugin must pass (or skip) the execution. The cluster upgrade plugin is responsible to schedule the upgrade conformance suite, which will upgrade the cluster while running conformance suite to monitor upgrade. This plugin is enabled when the execution mode is <code>upgrade</code>.",
			Expected:     "The cluster upgrade plugin must pass the execution when execution mode is <code>upgrade</code>.",
			Action:       "Check the cluster upgrade logs (click under the test name in the job list). If the cause is undefined, re-run the execution or raise a question.",
			Troubleshoot: "Review the cluster upgrade logs and check the artifacts generated by the plugin.",
		},
	})
	// TODO(etcd)
	/*
		checkSum.Checks = append(checkSum.Checks, &Check{
			Name: "[TODO] etcd fio must accept the tests (TODO)",
			Test: AcceptanceCheckFail,
		})
	*/
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-010A",
		Name: "etcd logs: slow requests: average should be under 500ms",
		Test: func() CheckResult {
			prefix := "Check OPCT-010A Failed"
			wantLimit := 500.0
			res := CheckResult{
				Name:   CheckResultNameFail,
				Target: fmt.Sprintf("<=%.2f ms", wantLimit),
				Actual: "N/A",
			}
			if re.Provider == nil {
				log.Debugf("%s: unable to read provider information.", prefix)
				return res
			}
			if re.Provider.MustGatherInfo == nil {
				res.Actual = "ERR !must-gather"
				log.Debugf("%s: unable to read must-gather information.", prefix)
				return res
			}
			if re.Provider.MustGatherInfo.ErrorEtcdLogs == nil {
				res.Actual = "ERR !logs"
				log.Debugf("%s: unable to etcd stat from must-gather.", prefix)
				return res
			}
			if re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"] == nil {
				res.Actual = "ERR !counters"
				log.Debugf("%s: unable to read statistics from parsed etcd logs.", prefix)
				return res
			}
			if re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"].StatMean == "" {
				res.Actual = "ERR !p50"
				log.Debugf("%s: unable to get p50/mean statistics from parsed data: %v", prefix, re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"])
				return res
			}
			values := strings.Split(re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"].StatMean, " ")
			if values[0] == "" {
				log.Debugf("%s: unable to get parse p50/mean: %v", prefix, values)
				return res
			}
			value, err := strconv.ParseFloat(values[0], 64)
			if err != nil {
				log.Debugf("%s: unable to convert p50/mean to float: %v", prefix, err)
				return res
			}
			res.Actual = fmt.Sprintf("%.3f", value)
			if value >= wantLimit {
				log.Debugf("%s acceptance criteria: want=[<%.0f] got=[%v]", prefix, wantLimit, value)
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: `The etcd logs must generate the average of slow requests lower than 500 milisseconds.
The slow requests are a metric that helps to understand the health of the etcd. The slow requests are a relative value
and it is based on the observed values in known, and tested, cloud providers/platforms.`,
			Action:   `Review if the storage volume for control plane nodes, or dedicated volume for etcd, has the required performance to run etcd in production environment.`,
			Expected: `The slow requests in etcd logs are a relative value and it is based on the observed values in known platforms.`,
			Troubleshoot: `
1) Review the documentation for the required storage for etcd:

- A) [Product Documentation](https://docs.openshift.com/container-platform/4.13/installing/installing_platform_agnostic/installing-platform-agnostic.html#installation-minimum-resource-requirements_installing-platform-agnostic)
- B) [Red Hat Article: Understanding etcd and the tunables/conditions affecting performance](https://access.redhat.com/articles/7010406#effects-of-network-latency--jitter-on-etcd-4)
- C) [Red Hat Article: How to Use 'fio' to Check Etcd Disk Performance in OCP](https://access.redhat.com/solutions/4885641)
- D) [etcd-operator: baseline speed for standard hardware](https://github.com/openshift/cluster-etcd-operator/blob/f68835306c2d6670697a5fd98ba8c6ffe197ab02/pkg/hwspeedhelpers/hwhelper.go#L21-L34)

2) Check the performance described in the article(B)

3) Review the processed values from your environment

!!! danger "Requirement"
	It is required to run a conformance validation in a new cluster.

	The validation tests parses the etcd logs from the entire cluster, including historical data, if you changed
	the storage and didn't recreate the cluster, the results will include values containing slow requests from the
	old storage, impacting in the current view.

Run the report with debug flag <code>--loglevel=debug</code>:
~~~text
(...)
DEBU[2023-09-25T12:52:05-03:00] Check OPCT-010 Failed Acceptance criteria: want=[<500] got=[690.412] 
DEBU[2023-09-25T12:52:05-03:00] Check OPCT-011 Failed Acceptance criteria: want=[<1000] got=[3091.49]
~~~

Extract the information from the logs using parser utility:

~~~sh
# Export the path of extracted must-gather. Example:
export MUST_GATHER_PATH=${PWD}/must-gather.local.2905984348081335046

# Run the utility
cat ${MUST_GATHER_PATH}/*/namespaces/openshift-etcd/pods/*/etcd/etcd/logs/current.log \
	| opct adm parse-etcd-logs --aggregator hour

# Or, use the must-gather path
opct adm parse-etcd-logs --aggregator hour --path ${MUST_GATHER_PATH}
~~~

References:

- [etcd: Hardware recommendations](https://etcd.io/docs/v3.5/op-guide/hardware/)
- [OpenShift Docs: Planning your environment according to object maximums](https://docs.openshift.com/container-platform/4.11/scalability_and_performance/planning-your-environment-according-to-object-maximums.html)
- [OpenShift KCS: Backend Performance Requirements for OpenShift etcd](https://access.redhat.com/solutions/4770281)
- [IBM: Using Fio to Tell Whether Your Storage is Fast Enough for Etcd](https://www.ibm.com/cloud/blog/using-fio-to-tell-whether-your-storage-is-fast-enough-for-etcd)`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-010B",
		Name: "etcd logs: slow requests: maximum should be under 1000ms",
		Test: func() CheckResult {
			prefix := "Check OPCT-010B Failed"
			wantLimit := 1000.0
			res := CheckResult{
				Name:   CheckResultNameFail,
				Target: fmt.Sprintf("<=%.2f ms", wantLimit),
				Actual: "N/A",
			}
			if re.Provider.MustGatherInfo == nil {
				res.Actual = "ERR !must-gather"
				log.Debugf("%s: unable to read must-gather information.", prefix)
				return res
			}
			if re.Provider.MustGatherInfo.ErrorEtcdLogs == nil {
				res.Actual = "ERR !logs"
				log.Debugf("%s: unable to etcd stat from must-gather.", prefix)
				return res
			}
			if re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"] == nil {
				res.Actual = "ERR !counters"
				log.Debugf("%s: unable to read statistics from parsed etcd logs.", prefix)
				return res
			}
			if re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"].StatMax == "" {
				res.Actual = "ERR !max"
				log.Debugf("%s: unable to get max statistics from parsed data: %v", prefix, re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"])
				return res
			}
			values := strings.Split(re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"].StatMax, " ")
			if values[0] == "" {
				res.Actual = "ERR !max"
				log.Debugf("%s: unable to get parse max: %v", prefix, values)
				return res
			}
			value, err := strconv.ParseFloat(values[0], 64)
			if err != nil {
				res.Actual = "ERR !max"
				log.Debugf("%s: unable to convert max to float: %v", prefix, err)
				return res
			}
			res.Actual = fmt.Sprintf("%.3f", value)
			if value >= wantLimit {
				log.Debugf("%s acceptance criteria: want=[<%.0f] got=[%v]", prefix, wantLimit, value)
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: `The etcd logs must generate the maximum of slow requests lower than 1000 milisseconds.
One or more requests with high latency could impact the cluster performance. Slow requests are a metric that helps to
understand the health of the etcd. The slow requests are a relative value and it is based on the observed values in known platforms.
The maximum value is the highest value of slow requests reported in the etcd logs, it must not be higher than 1 second.
`,
			Action:       "Review if the storage volume for control plane nodes, or dedicated volume for etcd, has the required performance to run etcd in production environment.",
			Expected:     "The slow requests in etcd logs are a relative value and it is based on the observed values in known platforms.",
			Troubleshoot: "Review Dependencies: [Troubleshooting section of OPCT-010A](#opct-010A)",
			Dependencies: []string{"OPCT-010A"},
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   CheckID022,
		Name: "Detected one or more plugin(s) with potential invalid result",
		Test: func() CheckResult {
			prefix := "Check Failed - " + CheckID022

			res := CheckResult{Name: CheckResultNameFail, Target: "passed", Actual: "N/A"}
			checkPlugins := []string{
				plugin.PluginNameKubernetesConformance,
				plugin.PluginNameOpenShiftConformance,
				plugin.PluginNameArtifactsCollector,
			}
			invalidPluginIds := []string{}
			for _, plugin := range checkPlugins {
				if _, ok := re.Provider.Plugins[plugin]; !ok {
					return res
				}
				p := re.Provider.Plugins[plugin]
				if p.Stat.Total == p.Stat.Failed {
					log.Debugf("%s Runtime: Total and Failed counters are equals indicating execution failure", prefix)
					invalidPluginIds = append(invalidPluginIds, strings.Split(plugin, "-")[0])
				}
			}

			if len(invalidPluginIds) > 0 {
				res.Actual = fmt.Sprintf("Failed%v", invalidPluginIds)
				return res
			}

			res.Name = CheckResultNamePass
			res.Actual = "passed"
			log.Debugf("%s: %s", prefix, msgDefaultNotMatch)
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: `The plugin(s) must pass the execution, or generate valid results.
The plugin(s) are responsible to execute the conformance test suites, and generate the report.`,
			Action:   "Check the plugin logs (click under the test name in the job list). If the cause is undefined, re-run the execution",
			Expected: "The plugin(s) must pass the execution, or generate valid results.",
			Troubleshoot: `Review the plugin logs and check the artifacts generated by the plugin.
Possible causes of failed plugins:
	- The plugin is not able to execute the tests: Check the plugin logs for errors in the directory "plugins" in the report archive
	- The plugin total counter is equal than the failed counter: Check the output of 'opct report' indicating the failed plugins`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID: CheckID023A,
		// Should be greated than 300
		Name: "Sanity [10-openshift-kube-conformance]: potential missing tests in suite",
		Test: func() CheckResult {
			prefix := "Check Failed - " + CheckID023A
			res := CheckResult{
				Name:   CheckResultNameFail,
				Target: "F:<300",
				Actual: "N/A",
			}
			if _, ok := re.Provider.Plugins[plugin.PluginNameKubernetesConformance]; !ok {
				res.Actual = "ERR !plugin"
				return res
			}
			p := re.Provider.Plugins[plugin.PluginNameKubernetesConformance]
			res.Actual = fmt.Sprintf("Total==%d", p.Stat.Total)
			if p.Stat.Total <= 300 {
				log.Debugf("%s: found less than expected tests count=%d. Are you running in devel mode?", prefix, p.Stat.Total)
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: `The Kubernetes Conformance suite must have acceptable number of tests to be considered as a valid execution.`,
			Expected: `The Kubernetes Conformance suite must have at least 300 tests to be valid. This number is based in the kubernetes
conformance suite across different releases. This test is a sanity test to ensure that the plugin is running correctly.`,
			Troubleshoot: "Review the plugin logs and check the artifacts generated by the plugin.",
			Action:       "This is unexpected for regular cluster validation. Check the plugin logs and the artifacts generated by the <code>opct report</code> to check if the job for Kubernetes Conformance suite have been completed.",
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID: CheckID023B,
		// Should be greated than 3000
		Name: "Sanity [20-openshift-conformance-validated]: potential missing tests in suite",
		Test: func() CheckResult {
			prefix := "Check Failed - " + CheckID023B
			res := CheckResult{
				Name:   CheckResultNameFail,
				Target: "F:<3000",
				Actual: "N/A",
			}
			if _, ok := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]; !ok {
				res.Actual = "ERR !plugin"
				return res
			}
			p := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]
			res.Actual = fmt.Sprintf("Total==%d", p.Stat.Total)
			if p.Stat.Total <= 3000 {
				log.Debugf("%s: found less than expected tests count=%d. Is it running in devel mode?!", prefix, p.Stat.Total)
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: `The OpenShift Conformance suite must have acceptable number of tests to be considered as a valid execution.`,
			Expected: `The OpenShift Conformance suite must have at least 3000 tests to be valid. This number is based in the OpenShift
conformance suite across different releases. This test is a sanity test to ensure that the plugin is running correctly.`,
			//Troubleshoot: ``,
			Action: `Review the plugin logs and check the artifacts generated by the plugin.`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-030",
		Name: "Node Topology: ControlPlaneTopology HighlyAvailable must use multi-zone",
		Test: func() CheckResult {
			prefix := "Check OPCT-030 Failed"
			res := CheckResult{
				Name:   CheckResultNameFail,
				Target: "W:>1,P:>2",
				Actual: "N/A",
			}
			if re.Provider.Infra == nil {
				log.Debugf("%s: missing Infrastructure object to discover ControlPlaneTopology", prefix)
				res.Actual = "ERR !infra"
				return res
			}
			if re.Provider.Infra.ControlPlaneTopology != "HighlyAvailable" {
				res.Name = CheckResultNameSkip
				res.Actual = fmt.Sprintf("Topology==%s", re.Provider.Infra.ControlPlaneTopology)
				return res
			}
			// Skip when topology isn't available (no-Cloud provider information)
			provider := re.Provider.Infra.PlatformType
			if re.Provider.Infra.PlatformType == "None" {
				res.Name = CheckResultNameSkip
				res.Actual = fmt.Sprintf("Type==%s", provider)
				return res
			}
			// Why having 2 or less nodes in HighlyAvailable?
			if len(re.Provider.Nodes) < 3 {
				log.Debugf("%s: two or less control plane nodes", prefix)
				res.Actual = fmt.Sprintf("Nodes==%d", len(re.Provider.Nodes))
				return res
			}
			controlPlaneZones := map[string]struct{}{}
			for _, node := range re.Provider.Nodes {
				if !node.ControlPlane {
					continue
				}
				if zone, ok := node.Labels["topology.kubernetes.io/zone"]; ok {
					controlPlaneZones[zone] = struct{}{}
				}
			}
			if len(controlPlaneZones) < 2 {
				log.Debugf("%s: found one zone: %v", prefix, controlPlaneZones)
				res.Actual = fmt.Sprintf("Zones==%d", len(controlPlaneZones))
				return res
			}
			res.Name = CheckResultNamePass
			res.Actual = fmt.Sprintf("Zones==%d", len(controlPlaneZones))
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description:  `The control plane nodes must be distributed across multiple zones to ensure high availability.`,
			Expected:     `The control plane nodes must be distributed across multiple zones to ensure high availability.`,
			Troubleshoot: ``,
			Action:       `Check the control plane nodes and ensure that the nodes are distributed across multiple zones.`,
		},
	})
	// OpenShift / Infrastructure Object Check
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   CheckIdEmptyValue,
		Name: "Platform Type must be supported by OPCT",
		Test: func() CheckResult {
			prefix := "Check OPCT-TBD Failed"
			res := CheckResult{Name: CheckResultNameFail, Target: "None|External|AWS|Azure"}
			if re.Provider == nil || re.Provider.Infra == nil {
				res.Message = fmt.Sprintf("%s: unable to read the infrastructure object", prefix)
				log.Debug(res.Message)
				return res
			}
			// Acceptance Criteria
			res.Actual = re.Provider.Infra.PlatformType
			switch res.Actual {
			case "None", "External", "AWS", "Azure":
				res.Name = CheckResultNamePass
				return res
			}
			log.Debugf("%s (Platform Type): %s: got=[%s]", prefix, msgDefaultNotMatch, re.Provider.Infra.PlatformType)
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description: `The platform type must be supported by the OPCT tool to generate valid and tested reports.
You can run the conformance tests in different platforms, but the OPCT results is tested with specific platforms, and the
report is made and calibrated based in the tested platforms.`,
			Expected:     `The platform type must be supported by the OPCT tool to generate valid and tested reports.`,
			Troubleshoot: `Review the platform type in the report and check the artifacts generated by the plugin: oc get infrastructure`,
			Action:       `Check the platform type in the report and ensure that the platform is supported by the OPCT tool.`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   CheckIdEmptyValue,
		Name: "Cluster Version Operator must be Available",
		Test: func() CheckResult {
			res := CheckResult{Name: CheckResultNameFail, Target: "True"}
			prefix := "Check Failed"
			if re.Provider == nil || re.Provider.Version == nil || re.Provider.Version.OpenShift == nil {
				res.Message = fmt.Sprintf("%s: unable to read provider version", prefix)
				return res
			}
			res.Actual = re.Provider.Version.OpenShift.CondAvailable
			if res.Actual != "True" {
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description:  `The Cluster Version Operator must be available to ensure that the cluster is in a healthy state.`,
			Expected:     `The Cluster Version Operator must be available to ensure that the cluster is in a healthy state.`,
			Troubleshoot: `Review the Cluster Version Operator logs and check the artifacts generated by the plugin.`,
			Action: `Check the Cluster Version Operator logs (click under the test name in the job list). If the cause is undefined, re-run the execution
and check the logs for errors.`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   CheckIdEmptyValue,
		Name: "Cluster condition Failing must be False",
		Test: func() CheckResult {
			res := CheckResult{Name: CheckResultNameFail, Target: "False"}
			prefix := "Check Failed"
			if re.Provider == nil || re.Provider.Version == nil || re.Provider.Version.OpenShift == nil {
				res.Message = fmt.Sprintf("%s: unable to read provider version", prefix)
				return res
			}
			res.Actual = re.Provider.Version.OpenShift.CondFailing
			if res.Actual != "False" {
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description:  `The Cluster condition Failing must be False to ensure that the cluster is in a healthy state.`,
			Expected:     `The Cluster condition Failing must be False to ensure that the cluster is in a healthy state.`,
			Troubleshoot: `Review the Cluster condition Failing logs and check the artifacts generated by the plugin.`,
			Action: `Check the Cluster condition Failing logs (click under the test name in the job list). If the cause is undefined, re-run the execution
and check the logs for errors.`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   CheckIdEmptyValue,
		Name: "Cluster upgrade must not be Progressing",
		Test: func() CheckResult {
			res := CheckResult{Name: CheckResultNameFail, Target: "False"}
			if re.Provider == nil || re.Provider.Version == nil || re.Provider.Version.OpenShift == nil {
				return res
			}
			res.Actual = re.Provider.Version.OpenShift.CondProgressing
			if res.Actual != "False" {
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description:  `The Cluster upgrade must not be Progressing to ensure that the cluster is in a healthy state.`,
			Expected:     `The Cluster upgrade must not be Progressing to ensure that the cluster is in a healthy state.`,
			Troubleshoot: `Review the Cluster upgrade logs and check the artifacts generated by the plugin.`,
			Action: `Check the Cluster upgrade logs (click under the test name in the job list). If the cause is undefined, re-run the execution
and check the logs for errors.`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   CheckIdEmptyValue,
		Name: "Cluster ReleaseAccepted must be True",
		Test: func() CheckResult {
			res := CheckResult{Name: CheckResultNameFail, Target: "True"}
			if re.Provider == nil || re.Provider.Version == nil || re.Provider.Version.OpenShift == nil {
				return res
			}
			res.Actual = re.Provider.Version.OpenShift.CondReleaseAccepted
			if res.Actual != "True" {
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description:  `The Cluster ReleaseAccepted must be True to ensure that the cluster is in a healthy state.`,
			Expected:     `The Cluster ReleaseAccepted must be True to ensure that the cluster is in a healthy state.`,
			Troubleshoot: `Review the Cluster ReleaseAccepted logs and check the artifacts generated by the plugin.`,
			Action: `Check the Cluster ReleaseAccepted logs (click under the test name in the job list). If the cause is undefined, re-run the execution
and check the logs for errors.`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   CheckIdEmptyValue,
		Name: "Infrastructure status must have Topology=HighlyAvailable",
		Test: func() CheckResult {
			res := CheckResult{Name: CheckResultNameFail, Target: "HighlyAvailable"}
			if re.Provider == nil || re.Provider.Infra == nil {
				return res
			}
			res.Actual = re.Provider.Infra.Topology
			if res.Actual != "HighlyAvailable" {
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description:  `The infrastructure status must have Topology=HighlyAvailable to ensure that the cluster is in a healthy state.`,
			Expected:     `The infrastructure status must have Topology=HighlyAvailable to ensure that the cluster is in a healthy state.`,
			Troubleshoot: `Review the infrastructure status logs and check the artifacts generated by the plugin.`,
			Action: `Check the infrastructure status logs (click under the test name in the job list). If the cause is undefined, re-run the execution
and check the logs for errors.`,
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   CheckIdEmptyValue,
		Name: "Infrastructure status must have ControlPlaneTopology=HighlyAvailable",
		Test: func() CheckResult {
			res := CheckResult{Name: CheckResultNameFail, Target: "HighlyAvailable"}
			if re.Provider == nil || re.Provider.Infra == nil {
				return res
			}
			res.Actual = re.Provider.Infra.ControlPlaneTopology
			if re.Provider.Infra.ControlPlaneTopology != "HighlyAvailable" {
				return res
			}
			res.Name = CheckResultNamePass
			return res
		},
		DocumentationSpec: CheckDocumentationSpec{
			Description:  `The infrastructure status must have ControlPlaneTopology=HighlyAvailable to ensure that the cluster is in a healthy state.`,
			Expected:     `The infrastructure status must have ControlPlaneTopology=HighlyAvailable to ensure that the cluster is in a healthy state.`,
			Troubleshoot: `Review the infrastructure status logs and check the artifacts generated by the plugin.`,
			Action: `Check the infrastructure status logs (click under the test name in the job list). If the cause is undefined, re-run the execution
and check the logs for errors.`,
		},
	})
	// TODO(network): podConnectivityChecks must not have outages

	// TODO:
	// Question#1: Do we need this test considering there is a check of passing=100% on kube conformance?
	// Question#2: is that check really need considering the final filters target 0 failures?
	// checkSum.Checks = append(checkSum.Checks, &Check{
	// 	ID: "OPCT-TBD",
	// 	Name:        "Kubernetes Conformance [10-openshift-kube-conformance]: replay failures must-pass",
	// 	Description: "Tests that failed in the previous run must pass in the replay step (re-run)",
	// 	Test: func() CheckResult {
	// 		return CheckResult{Name: CheckResultNameSkip, Target: "TBD", Actual: "TODO"}
	// 	},
	// })
	// checkSum.Checks = append(checkSum.Checks, &Check{
	// 	ID:          "OPCT-TBD",
	// 	Name:        "OpenShift Conformance [20-openshift-conformance-validated]: replay failures must-pass",
	// 	Description: "Tests that failed in the previous run must pass in the replay step (re-run)",
	// 	Test: func() CheckResult {
	// 		// for each failed test in the Filter5, check if it passed in the replay.
	// 		// return CheckResult{Name: CheckResultNameSkip, Target: "TBD", Actual: "TODO"}
	// 		res := CheckResult{
	// 			Name:   CheckResultNameFail,
	// 			Target: "F:<300",
	// 			Actual: "N/A",
	// 		}
	// 	},
	// })

	// Create docs reference when ID is set
	for c := range checkSum.Checks {
		if checkSum.Checks[c].ID != CheckIdEmptyValue {
			checkSum.Checks[c].Documentation = fmt.Sprintf("%s/#%s", checkSum.baseURL, checkSum.Checks[c].ID)
		}
	}
	return checkSum
}

func (csum *CheckSummary) GetBaseURL() string {
	return csum.baseURL
}

func (csum *CheckSummary) GetCheckResults() ([]*SLOOutput, []*SLOOutput, []*SLOOutput, []*SLOOutput) {
	passes := []*SLOOutput{}
	failures := []*SLOOutput{}
	warnings := []*SLOOutput{}
	skips := []*SLOOutput{}
	for _, check := range csum.Checks {
		if check.Result.String() == string(CheckResultNameFail) {
			failures = append(failures, &SLOOutput{
				ID:            check.ID,
				SLO:           check.Name,
				SLOResult:     check.Result.String(),
				SLITarget:     check.Result.Target,
				SLIActual:     check.Result.Actual,
				Message:       check.Result.Message,
				Documentation: check.Documentation,
			})
		} else if check.Result.String() == string(CheckResultNameWarn) {
			warnings = append(warnings, &SLOOutput{
				ID:            check.ID,
				SLO:           check.Name,
				SLOResult:     check.Result.String(),
				SLITarget:     check.Result.Target,
				SLIActual:     check.Result.Actual,
				Message:       check.Result.Message,
				Documentation: check.Documentation,
			})
		} else if check.Result.String() == string(CheckResultNameSkip) {
			skips = append(skips, &SLOOutput{
				ID:            check.ID,
				SLO:           check.Name,
				SLOResult:     check.Result.String(),
				SLITarget:     check.Result.Target,
				SLIActual:     check.Result.Actual,
				Message:       check.Result.Message,
				Documentation: check.Documentation,
			})
		} else {
			passes = append(passes, &SLOOutput{
				ID:            check.ID,
				SLO:           check.Name,
				SLOResult:     check.Result.String(),
				SLITarget:     check.Result.Target,
				SLIActual:     check.Result.Actual,
				Message:       check.Result.Message,
				Documentation: check.Documentation,
			})
		}
	}
	return passes, failures, warnings, skips
}

func (csum *CheckSummary) Run() error {
	for _, check := range csum.Checks {
		check.Result = check.Test()
	}
	return nil
}

// CheckDocumentationSpec generates the markdown documentation for all rules/checks/SLOs.
func (csum *CheckSummary) generateDocumentation() string {
	var doc strings.Builder

	// Create the header
	doc.WriteString(`# OPCT Review/Check Rules

The OPCT rules are used in the report command to evaluate the data collected by the OPCT execution.
The HTML report will link directly to the rule ID on this page.

The rule details can be used as an additional resource in the review process.

The acceptance criteria for the rules are based on multiple CI jobs used as a reference to evaluate the expected result.
If you have any questions about the rules, please file an Issue in the OPCT repository.

## Rules
___

`)

	// sort csum.Checks by check.ID to ensure the order is consistent. The empty IDs or "--" will be at the end
	sort.Slice(csum.Checks, func(i, j int) bool {
		if csum.Checks[i].ID == CheckIdEmptyValue {
			return false
		}
		return csum.Checks[i].ID < csum.Checks[j].ID
	})

	// Create the documentation for each check
	for _, check := range csum.Checks {
		if check.ID == CheckIdEmptyValue {
			doc.WriteString(fmt.Sprintf("### %s\n\n", check.Name))
		} else {
			doc.WriteString(fmt.Sprintf("### %s\n\n", check.ID))
		}
		doc.WriteString(fmt.Sprintf("- **Name**: %s\n", check.Name))
		doc.WriteString(fmt.Sprintf("- **Description**: %s\n", check.DocumentationSpec.Description))
		doc.WriteString(fmt.Sprintf("- **Action**: %s\n", check.DocumentationSpec.Action))
		if len(check.DocumentationSpec.Expected) > 0 {
			doc.WriteString(fmt.Sprintf("- **Expected**:\n%s\n", check.DocumentationSpec.Expected))
		}
		if len(check.DocumentationSpec.Troubleshoot) > 0 {
			doc.WriteString(fmt.Sprintf("- **Troubleshoot**:\n%s\n", check.DocumentationSpec.Troubleshoot))
		}
		if len(check.DocumentationSpec.Dependencies) > 0 {
			var deps strings.Builder
			for idx, dep := range check.DocumentationSpec.Dependencies {
				if idx > 0 {
					deps.WriteString(", ")
				}
				deps.WriteString(fmt.Sprintf("[%s](#%s)", dep, strings.ToLower(dep)))
			}
			doc.WriteString(fmt.Sprintf("- **Dependencies**: %s\n", deps.String()))
		}
		doc.WriteString("\n")
	}

	doc.WriteString("___\n")
	doc.WriteString(`*<p style='text-align:center;'>Page generated automatically by <code>opct adm generate checks-docs</code></p>*`)

	return doc.String()
}

// WriteDocumentation writes the documentation to the provided path.
func (csum *CheckSummary) WriteDocumentation(docPath string) error {
	doc := csum.generateDocumentation()
	return os.WriteFile(docPath, []byte(doc), 0644)
}
