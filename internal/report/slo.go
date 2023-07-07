// Description: This file contains the implementation of the SLO interface,
// translated to "checks" in the OPCT report package. The SLO interface is defined
// in the report package, and the package implements SLIs to ensure acceptance
// criteria is met in the data collected from artifacts.
// Reference: https://github.com/kubernetes/community/blob/master/sig-scalability/slos/slos.md
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

	CheckResultNamePass CheckResultName = "pass"
	CheckResultNameFail CheckResultName = "fail"
	CheckResultNameWarn CheckResultName = "warn"
	CheckResultNameSkip CheckResultName = "skip"

	CheckIdEmptyValue string = "--"

	// SLOs
	CheckID001 string = "OPCT-001"
	CheckID004 string = "OPCT-004"
	CheckID005 string = "OPCT-005"
	CheckID022 string = "OPCT-022"
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
			res.Actual = fmt.Sprintf("Priority==%d", len(p.TestsFailedPrio))
			if len(p.TestsFailedPrio) > 0 {
				log.Debugf("%s Acceptance criteria: TestsFailedPrio counter are greater than 0: %v", prefix, len(p.TestsFailedPrio))
				return res
			}
			res.Name = CheckResultNamePass
			return res
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
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-005B",
		Name: "OpenShift Conformance Validation [20]: Required to Pass After Filtering",
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
	})
	// TODO: validate if this test is duplicated with OPCT-005
	// checkSum.Checks = append(checkSum.Checks, &Check{
	// 	ID:   "OPCT-TBD",
	// 	Name: "OpenShift Conformance [20-openshift-conformance-validated]: Pass 100% with Baseline",
	// 	Test: func() CheckResult {
	// 		prefix := "Check OPCT-TBD Failed"
	// 		res := CheckResult{
	// 			Name:   CheckResultNameFail,
	// 			Target: "Pass==100%",
	// 			Actual: "N/A",
	// 		}
	// 		if _, ok := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]; !ok {
	// 			return res
	// 		}
	// 		if re.Baseline == nil {
	// 			res.Name = CheckResultNameSkip
	// 			return res
	// 		}
	// 		if _, ok := re.Baseline.Plugins[plugin.PluginNameOpenShiftConformance]; !ok {
	// 			res.Name = CheckResultNameSkip
	// 			return res
	// 		}
	// 		// "Acceptance" are relative, the baselines is observed to set
	// 		// an "accepted" value considering a healthy cluster in known provider/installation.
	// 		p := re.Provider.Plugins[plugin.PluginNameOpenShiftConformance]
	// 		if p.Stat.Total == p.Stat.Failed {
	// 			res.Message = "Potential Runtime Failure. Check the Plugin logs."
	// 			res.Actual = "Total==Failed"
	// 			log.Debugf("%s Runtime: Total and Failed counters are equals indicating execution failure", prefix)
	// 			return res
	// 		}
	// 		perc := (float64(p.Stat.FilterFailedPrio) / float64(p.Stat.Total)) * 100
	// 		res.Actual = fmt.Sprintf("FailedPrio==%.2f%%", perc)
	// 		if perc > 0 {
	// 			res.Name = CheckResultNameFail
	// 			return res
	// 		}
	// 		res.Name = CheckResultNamePass
	// 		return res
	// 	},
	// })

	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-011",
		Name: "The test suite should generate fewer error reports in the logs",
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
	})
	checkSum.Checks = append(checkSum.Checks, &Check{
		ID:   "OPCT-010",
		Name: "The cluster logs should generate fewer error reports in the logs",
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
