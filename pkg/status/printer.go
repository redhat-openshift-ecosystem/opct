package status

import (
	"fmt"
	"os"
	"sort"
	"text/template"
	"time"

	"github.com/redhat-openshift-ecosystem/opct/pkg"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/aggregation"
)

type PrintableStatus struct {
	GlobalStatus   string
	CurrentTime    string
	ElapsedTime    string
	PluginStatuses []PrintablePluginStatus
}

type PrintablePluginStatus struct {
	Name     string
	Status   string
	Result   string
	Progress string
	Message  string
}

var runningStatusTemplate = `{{.CurrentTime}}|{{.ElapsedTime}}> Global Status: {{.GlobalStatus}}
{{printf "%-34s | %-10s | %-10s | %-25s | %-50s" "JOB_NAME" "STATUS" "RESULTS" "PROGRESS" "MESSAGE"}}{{range $index, $pl := .PluginStatuses}}
{{printf "%-34s | %-10s | %-10s | %-25s | %-50s" $pl.Name $pl.Status $pl.Result $pl.Progress $pl.Message}}{{end}}
`

func (s *Status) printRunningStatus() error {
	statusTemplate, err := template.New("statusTemplate").Parse(runningStatusTemplate)
	if err != nil {
		return err
	}
	return statusTemplate.Execute(os.Stdout, s.getPrintableRunningStatus())
}

func (s *Status) getPrintableRunningStatus() PrintableStatus {
	now := time.Now()
	ps := PrintableStatus{
		GlobalStatus: s.Latest.Status,
		CurrentTime:  now.Format(time.RFC1123),
		ElapsedTime:  now.Sub(s.StartTime).Truncate(time.Second).String(),
	}

	for _, pl := range s.Latest.Plugins {
		var progress string
		var message string

		if pl.Progress != nil {
			progress = fmt.Sprintf("%d/%d (%d failures)", pl.Progress.Completed, pl.Progress.Total, len(pl.Progress.Failures))
		}
		// Get PodStatus from the plugin when progress API is not available, allowing a
		// better visibility when issues to schedule jobs.
		if len(progress) == 0 {
			pod, err := getPluginPod(s.kclient, pkg.CertificationNamespace, pl.Plugin)
			var podStatus string
			if err != nil {
				podStatus = err.Error()
			} else {
				podStatus = getPodStatusString(pod)
				// If pod is in NotReady or error state, append error details
				if podStatus == "NotReady" || podStatus == "Pending" || podStatus == "Failed" {
					// First try to get container failure details
					if containerMsg := getPodContainerFailureMessage(pod); containerMsg != "" {
						podStatus = fmt.Sprintf("%s (%s)", podStatus, containerMsg)
					} else if eventMsg := getPodEventsMessage(s.kclient, pkg.CertificationNamespace, pod.Name); eventMsg != "" {
						// Fallback to events if no container details
						podStatus = fmt.Sprintf("%s (%s)", podStatus, eventMsg)
					}
				}
			}
			message = fmt.Sprintf("waiting for jobs initialization=PodStatus(%s)", podStatus)
		}

		if pl.Status == aggregation.RunningStatus {
			if pl.Progress != nil {
				message = pl.Progress.Message
			}
		} else if pl.ResultStatus == "" {
			message = "waiting for post-processor..."
			if pl.Status != "" {
				message = pl.Status
			}
		} else {
			// If we have results, print a summary of the results, otherwise just print the waiting message.
			passCount := pl.ResultStatusCounts["passed"]
			failedCount := pl.ResultStatusCounts["failed"]
			if passCount+failedCount != 0 {
				message = fmt.Sprintf("Total tests processed: %d (%d pass / %d failed)", passCount+failedCount, passCount, failedCount)
			}

			// If the plugin failed, try to get detailed error information
			if pl.Status == aggregation.FailedStatus {
				pod, err := getPluginPod(s.kclient, pkg.CertificationNamespace, pl.Plugin)
				if err == nil {
					var errorDetails string

					// First, check container termination status for direct failure reasons
					if containerMsg := getPodContainerFailureMessage(pod); containerMsg != "" {
						errorDetails = containerMsg
					} else {
						// Fallback to pod events if no container failure details
						if eventMsg := getPodEventsMessage(s.kclient, pkg.CertificationNamespace, pod.Name); eventMsg != "" {
							errorDetails = eventMsg
						}
					}

					if errorDetails != "" {
						// Append error details to the existing message or use as the message if empty
						if message != "" {
							message = fmt.Sprintf("%s | Error: %s", message, errorDetails)
						} else {
							message = fmt.Sprintf("Failed: %s", errorDetails)
						}
					}
				}
			}

		}

		pls := PrintablePluginStatus{
			Name:     pl.Plugin,
			Status:   pl.Status,
			Result:   pl.ResultStatus,
			Progress: progress,
			Message:  message,
		}
		ps.PluginStatuses = append(ps.PluginStatuses, pls)
	}

	sort.Slice(ps.PluginStatuses, func(i, j int) bool {
		return ps.PluginStatuses[i].Name < ps.PluginStatuses[j].Name
	})

	return ps
}
