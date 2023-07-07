/*
Checks handles all acceptance criteria from data
collected and processed in summary package.

Existing Checks:
- OPCT-001: "Plugin Conformance Kubernetes [10-openshift-kube-conformance] must pass (after filters)"
- OPCT-002: "Plugin Conformance Upgrade [05-openshift-cluster-upgrade] must pass"
- OPCT-003: "Plugin Collector [99-openshift-artifacts-collector] must pass"
- ...TBD
*/
package report

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/plugin"
	log "github.com/sirupsen/logrus"
)

const (
	docsRulesPath  = "/review/rules"
	defaultBaseURL = "https://redhat-openshift-ecosystem.github.io/provider-certification-tool"
)

type CheckSummary struct {
	baseURL string
	Checks  []*Check `json:"checks"`
}

func NewCheckSummary(re *Report) *CheckSummary {

	baseURL := defaultBaseURL
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
	// OpenShift / Infrastructure Object Check
	checkSum.Checks = append(checkSum.Checks, &Check{
		Name: "Platform Type should be None",
		Test: func() CheckResult {
			if re.Provider == nil || re.Provider.Infra == nil {
				// return CheckRespCustomFail("unable to access Infrastructure object")
				return CheckResultFail
			}
			if re.Provider.Infra.PlatformType != "None" {
				// return CheckRespCustomFail(fmt.Sprintf("PlatformType=%s", re.Provider.Infra.PlatformType))
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		Name: "Cluster Version Operator must be Available",
		Test: func() CheckResult {
			if re.Provider == nil || re.Provider.Version == nil || re.Provider.Version.OpenShift == nil {
				return CheckResultFail
			}
			if re.Provider.Version.OpenShift.CondAvailable != "True" {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		Name: "Cluster condition Failing must be False",
		Test: func() CheckResult {
			if re.Provider == nil || re.Provider.Version == nil || re.Provider.Version.OpenShift == nil {
				return CheckResultFail
			}
			if re.Provider.Version.OpenShift.CondFailing != "False" {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		Name: "Cluster upgrade must not be Progressing",
		Test: func() CheckResult {
			if re.Provider == nil || re.Provider.Version == nil || re.Provider.Version.OpenShift == nil {
				return CheckResultFail
			}
			if re.Provider.Version.OpenShift.CondProgressing != "False" {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		Name: "Cluster ReleaseAccepted must be True",
		Test: func() CheckResult {
			if re.Provider == nil || re.Provider.Version == nil || re.Provider.Version.OpenShift == nil {
				return CheckResultFail
			}
			if re.Provider.Version.OpenShift.CondReleaseAccepted != "True" {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		Name: "Infrastructure status must have Topology=HighlyAvailable",
		Test: func() CheckResult {
			if re.Provider == nil || re.Provider.Infra == nil {
				return CheckResultFail
			}
			if re.Provider.Infra.Topology != "HighlyAvailable" {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		Name: "Infrastructure status must have ControlPlaneTopology=HighlyAvailable",
		Test: func() CheckResult {
			if re.Provider == nil || re.Provider.Infra == nil {
				return CheckResultFail
			}
			if re.Provider.Infra.ControlPlaneTopology != "HighlyAvailable" {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-008",
		Name: "All nodes must be healthy",
		Test: func() CheckResult {
			if re.Provider == nil || re.Provider.ClusterHealth == nil {
				log.Debugf("Check Failed: OPCT-008: unavailable results")
				return CheckResultFail
			}
			if re.Provider.ClusterHealth.NodeHealthPerc != 100 {
				log.Debugf("Check Failed: OPCT-008: want[!=100] got[%f]", re.Provider.ClusterHealth.NodeHealthPerc)
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-009",
		Name: "Pods Healthy must report higher than 98%",
		Test: func() CheckResult {
			if re.Provider == nil || re.Provider.ClusterHealth == nil {
				return CheckResultFail
			}
			if re.Provider.ClusterHealth.PodHealthPerc < 98.0 {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	// Plugins Checks
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-001",
		Name: "Plugin Conformance Kubernetes [10-openshift-kube-conformance] must pass (after filters)",
		Test: func() CheckResult {
			if _, ok := re.Provider.Plugins[plugin.PluginNameKubernetesConformance]; !ok {
				return CheckResultFail
			}
			if len(re.Provider.Plugins[plugin.PluginNameKubernetesConformance].TestsFailedPrio) > 0 {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	// checkSum.Checks = append(checkSum.Checks, &Check{
	// 	Name: "OpenShift Conformance plugin 20-openshift-conformance-validated",
	// 	Test: func() CheckResult {
	// 		if _, ok := re.Provider.Plugins[PluginNameOpenShiftConformance]; !ok {
	// 			return CheckResultFail
	// 		}
	//      // "Acceptance" are relative, the baselines is observed to set
	//      // an "accepted" value considering a healthy cluster in known provider/installation.
	// 		plugin := re.Provider.Plugins[PluginNameOpenShiftConformance]
	// 		if re.Provider.ClusterHealth.PodHealthTotal != 0 {
	// 			return CheckResultFail
	// 		}
	// 		return CheckResultPass
	// 	},
	// })
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-004",
		Name: "OpenShift Conformance [20-openshift-conformance-validated]: Failed tests must report less than 1.5%",
		Test: func() CheckResult {
			if _, ok := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]; !ok {
				return CheckResultFail
			}
			// "Acceptance" are relative, the baselines is observed to set
			// an "accepted" value considering a healthy cluster in known provider/installation.
			plugin := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]
			perc := (float64(plugin.Stat.Failed) / float64(plugin.Stat.Total)) * 100
			if perc > 1.5 {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-005",
		Name: "OpenShift Conformance [20-openshift-conformance-validated]: Priority must report less than 0.5%",
		Test: func() CheckResult {
			if _, ok := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]; !ok {
				return CheckResultFail
			}
			// "Acceptance" are relative, the baselines is observed to set
			// an "accepted" value considering a healthy cluster in known provider/installation.
			plugin := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]
			perc := (float64(plugin.Stat.FilterFailedPrio) / float64(plugin.Stat.Total)) * 100
			if perc > 0.5 {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-006",
		Name: "Suite Errors must report a lower number of log errors",
		Test: func() CheckResult {
			if re.Provider.ErrorCounters == nil {
				return CheckResultFail
			}
			cnt := *re.Provider.ErrorCounters
			if _, ok := cnt["total"]; !ok {
				return CheckResultFail
			}
			// "Acceptance" are relative, the baselines is observed to set
			// an "accepted" value considering a healthy cluster in known provider/installation.
			total := cnt["total"]
			if total > 150 {
				return CheckResultFail
			}
			// 0? really? something went wrong!
			if total == 0 {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-007",
		Name: "Workloads must report a lower number of errors in the logs",
		Test: func() CheckResult {
			prefix := "Check OPCT-007 Failed"
			if re.Provider.MustGatherInfo == nil {
				log.Debugf("%s: MustGatherInfo is not defined", prefix)
				return CheckResultFail
			}
			if _, ok := re.Provider.MustGatherInfo.ErrorCounters["total"]; !ok {
				log.Debugf("%s: OPCT-007: ErrorCounters[\"total\"]", prefix)
				return CheckResultFail
			}
			// "Acceptance" are relative, the baselines is observed to set
			// an "accepted" value considering a healthy cluster in known provider/installation.
			total := re.Provider.MustGatherInfo.ErrorCounters["total"]
			wantLimit := 30000
			if total > wantLimit {
				log.Debugf("%s acceptance criteria: want[<=%d] got[%d]", prefix, wantLimit, total)
				return CheckResultFail
			}
			// 0? really? something went wrong!
			if total == 0 {
				log.Debugf("%s acceptance criteria: want[!=0] got[%d]", prefix, total)
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-003",
		Name: "Plugin Collector [99-openshift-artifacts-collector] must pass",
		Test: func() CheckResult {
			if _, ok := re.Provider.Plugins[plugin.PluginNameArtifactsCollector]; !ok {
				return CheckResultFail
			}
			if re.Provider.Plugins[plugin.PluginNameArtifactsCollector].Stat.Status != "passed" {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-002",
		Name: "Plugin Conformance Upgrade [05-openshift-cluster-upgrade] must pass",
		Test: func() CheckResult {
			if _, ok := re.Provider.Plugins[plugin.PluginNameOpenShiftUpgrade]; !ok {
				return CheckResultFail
			}
			if re.Provider.Plugins[plugin.PluginNameOpenShiftUpgrade].Stat.Status != "passed" {
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	// TODO(etcd)
	/*
		checkSum.Checks = append(checkSum.Checks, &Check{
			Name: "[TODO] etcd fio must accept the tests (TODO)",
			Test: AcceptanceCheckFail,
		})
		checkSum.Checks = append(checkSum.Checks, &Check{
			Name: "[TODO] etcd slow requests: p99 must be lower than 900ms",
			Test: AcceptanceCheckFail,
		})
	*/
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-010",
		Name: "etcd logs: slow requests: average should be under 500ms",
		Test: func() CheckResult {
			prefix := "Check OPCT-010 Failed"
			wantLimit := 500.0
			if re.Provider.MustGatherInfo == nil {
				log.Debugf("%s: unable to read must-gather information.", prefix)
				return CheckResultFail
			}
			if re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"] == nil {
				log.Debugf("%s: unable to read statistics from parsed etcd logs.", prefix)
				return CheckResultFail
			}
			if re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"].StatMean == "" {
				log.Debugf("%s: unable to get p50/mean statistics from parsed data: %v", prefix, re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"])
				return CheckResultFail
			}
			values := strings.Split(re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"].StatMean, " ")
			if values[0] == "" {
				log.Debugf("%s: unable to get parse p50/mean: %v", prefix, values)
				return CheckResultFail
			}
			value, err := strconv.ParseFloat(values[0], 64)
			if err != nil {
				log.Debugf("%s: unable to convert p50/mean to float: %v", prefix, err)
				return CheckResultFail
			}
			if value >= wantLimit {
				log.Debugf("%s acceptance criteria: want=[<%.0f] got=[%v]", prefix, wantLimit, value)
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-011",
		Name: "etcd logs: slow requests: maximum should be under 1000ms",
		Test: func() CheckResult {
			prefix := "Check OPCT-011 Failed"
			wantLimit := 1000.0
			if re.Provider.MustGatherInfo == nil {
				log.Debugf("%s: unable to read must-gather information.", prefix)
				return CheckResultFail
			}
			if re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"] == nil {
				log.Debugf("%s: unable to read statistics from parsed etcd logs.", prefix)
				return CheckResultFail
			}
			if re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"].StatMax == "" {
				log.Debugf("%s: unable to get max statistics from parsed data: %v", prefix, re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"])
				return CheckResultFail
			}
			values := strings.Split(re.Provider.MustGatherInfo.ErrorEtcdLogs.FilterRequestSlowAll["all"].StatMax, " ")
			if values[0] == "" {
				log.Debugf("%s: unable to get parse max: %v", prefix, values)
				return CheckResultFail
			}
			value, err := strconv.ParseFloat(values[0], 64)
			if err != nil {
				log.Debugf("%s: unable to convert max to float: %v", prefix, err)
				return CheckResultFail
			}
			if value >= wantLimit {
				log.Debugf("%s acceptance criteria: want=[<%.0f] got=[%v]", prefix, wantLimit, value)
				return CheckResultFail
			}
			return CheckResultPass
		},
	})
	// TODO(network): podConnectivityChecks must not have outages

	// Create docs reference when ID is set
	for c := range checkSum.Checks {
		if checkSum.Checks[c].ID != "" {
			checkSum.Checks[c].Reference = fmt.Sprintf("%s/#%s", checkSum.baseURL, checkSum.Checks[c].ID)
		}
	}
	return checkSum
}

func (csum *CheckSummary) GetBaseURL() string {
	return csum.baseURL
}

func (csum *CheckSummary) GetChecksFailed() []*Check {
	failures := []*Check{}
	for _, check := range csum.Checks {
		if check.Result == CheckResultFail {
			failures = append(failures, check)
		}
	}
	return failures
}

func (csum *CheckSummary) GetChecksPassed() []*Check {
	failures := []*Check{}
	for _, check := range csum.Checks {
		if check.Result == CheckResultPass {
			failures = append(failures, check)
		}
	}
	return failures
}

func (csum *CheckSummary) Run() error {
	for _, check := range csum.Checks {
		check.Result = check.Test()
	}
	return nil
}

type CheckResult string

const CheckResultPass = "pass"
const CheckResultFail = "fail"

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

	// Reference must point to documentation URL to review the
	// item.
	Reference string `json:"reference"`

	// Accepted must report acceptance criteria, when true
	// the Check is accepted by the tool, otherwise it is
	// failed and must be reviewede.
	Result CheckResult `json:"result"`

	ResultMessage string `json:"resultMessage"`

	Test func() CheckResult `json:"-"`
}

/* Checks implementation */

func ExampleAcceptanceCheckPass() CheckResult {
	return CheckResultPass
}

func AcceptanceCheckFail() CheckResult {
	return CheckResultFail
}

func CheckRespCustomFail(custom string) CheckResult {
	resp := CheckResult(fmt.Sprintf("%s [%s]", CheckResultFail, custom))
	return resp
}
