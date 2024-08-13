package status

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"sort"
	"time"

	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/aggregation"

	kcorev1 "k8s.io/api/core/v1"
	kmmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
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

func (s *StatusOptions) printRunningStatus() error {
	statusTemplate, err := template.New("statusTemplate").Parse(runningStatusTemplate)
	if err != nil {
		return err
	}
	return statusTemplate.Execute(os.Stdout, s.getPrintableRunningStatus())
}

func (s *StatusOptions) getPrintableRunningStatus() PrintableStatus {
	now := time.Now()
	ps := PrintableStatus{
		GlobalStatus: s.Latest.Status,
		CurrentTime:  now.Format(time.RFC1123),
		ElapsedTime:  now.Sub(s.StartTime).String(),
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

// GetPluginPod get the plugin pod spec.
func getPluginPod(kclient kubernetes.Interface, namespace string, pluginPodName string) (*kcorev1.Pod, error) {
	labelSelector := kmmetav1.LabelSelector{MatchLabels: map[string]string{"component": "sonobuoy", "sonobuoy-plugin": pluginPodName}}
	log.Debugf("Getting pod with labels: %v\n", labelSelector)
	listOptions := kmmetav1.ListOptions{
		LabelSelector: klabels.Set(labelSelector.MatchLabels).String(),
	}

	podList, err := kclient.CoreV1().Pods(namespace).List(context.TODO(), listOptions)
	if err != nil {
		return nil, fmt.Errorf("unable to list pods with label %q", labelSelector)
	}

	switch {
	case len(podList.Items) == 0:
		log.Warnf("no pods found with label %q in namespace %s", labelSelector, namespace)
		return nil, fmt.Errorf(fmt.Sprintf("no pods found with label %q in namespace %s", labelSelector, namespace))

	case len(podList.Items) > 1:
		log.Warnf("Found more than one pod with label %q. Using pod with name %q", labelSelector, podList.Items[0].GetName())
		return &podList.Items[0], nil
	default:
		return &podList.Items[0], nil
	}
}

// getPodStatusString get the pod status string.
func getPodStatusString(pod *kcorev1.Pod) string {
	if pod == nil {
		return "TBD(pod)"
	}

	for _, cond := range pod.Status.Conditions {
		// Pod Running
		if cond.Type == kcorev1.PodReady &&
			cond.Status == kcorev1.ConditionTrue &&
			pod.Status.Phase == kcorev1.PodRunning {
			return "Running"
		}
		// Pod Completed
		if cond.Type == kcorev1.PodReady &&
			cond.Status == "False" &&
			cond.Reason == "PodCompleted" {
			return "Completed"
		}
		// Pod NotReady (Container)
		if cond.Type == kcorev1.PodReady &&
			cond.Status == "False" &&
			cond.Reason == "ContainersNotReady" {
			return "NotReady"
		}
	}
	return string(pod.Status.Phase)
}
