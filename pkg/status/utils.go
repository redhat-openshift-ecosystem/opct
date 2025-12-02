package status

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	kcorev1 "k8s.io/api/core/v1"
	kmmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

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
		return nil, fmt.Errorf("no pods found with label %q in namespace %s", labelSelector, namespace)

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

// getPodEvents retrieves events associated with a pod, returning error-related events first
func getPodEvents(kclient kubernetes.Interface, namespace string, podName string) ([]kcorev1.Event, error) {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName)

	events, err := kclient.CoreV1().Events(namespace).List(context.TODO(), kmmetav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list events for pod %s: %w", podName, err)
	}

	// Filter and prioritize error/warning events
	var errorEvents []kcorev1.Event
	for _, event := range events.Items {
		if event.Type == kcorev1.EventTypeWarning || event.Reason == "Failed" || event.Reason == "BackOff" {
			errorEvents = append(errorEvents, event)
		}
	}

	return errorEvents, nil
}

// getPodEventsMessage returns a formatted message from the most recent error event
func getPodEventsMessage(kclient kubernetes.Interface, namespace string, podName string) string {
	events, err := getPodEvents(kclient, namespace, podName)
	if err != nil {
		log.Debugf("Unable to retrieve events for pod %s: %v", podName, err)
		return ""
	}

	if len(events) == 0 {
		return ""
	}

	// Return the most recent event message
	latestEvent := events[len(events)-1]
	return fmt.Sprintf("%s: %s", latestEvent.Reason, latestEvent.Message)
}

// getPodContainerFailureMessage extracts failure details from terminated containers
func getPodContainerFailureMessage(pod *kcorev1.Pod) string {
	if pod == nil {
		return ""
	}

	// Check all container statuses for termination reasons
	allStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)

	for _, containerStatus := range allStatuses {
		if containerStatus.State.Terminated != nil {
			terminated := containerStatus.State.Terminated

			// Container exited with non-zero code
			if terminated.ExitCode != 0 {
				reason := terminated.Reason
				if reason == "" {
					reason = "Error"
				}

				message := terminated.Message
				if message == "" {
					message = fmt.Sprintf("Container '%s' exited with code %d", containerStatus.Name, terminated.ExitCode)
				}

				return fmt.Sprintf("%s (exit %d): %s", reason, terminated.ExitCode, message)
			}
		}

		// Check waiting state for errors
		if containerStatus.State.Waiting != nil {
			waiting := containerStatus.State.Waiting
			if waiting.Reason != "" && waiting.Reason != "PodInitializing" {
				message := waiting.Message
				if message == "" {
					message = fmt.Sprintf("Container '%s' waiting", containerStatus.Name)
				}
				return fmt.Sprintf("%s: %s", waiting.Reason, message)
			}
		}
	}

	return ""
}
