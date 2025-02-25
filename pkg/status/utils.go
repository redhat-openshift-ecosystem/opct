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
