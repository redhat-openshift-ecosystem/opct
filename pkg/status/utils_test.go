package status

import (
	"testing"

	kcorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetPluginPod(t *testing.T) {
	kclient := fake.NewSimpleClientset(&kcorev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-namespace",
			Labels: map[string]string{
				"component":       "sonobuoy",
				"sonobuoy-plugin": "test-plugin",
			},
		},
	})

	namespace := "test-namespace"
	pluginPodName := "test-plugin"

	pod, err := getPluginPod(kclient, namespace, pluginPodName)
	if err != nil {
		t.Errorf("getPluginPod() returned an error: %v", err)
	}

	expectedPodName := "test-pod"
	if pod.Name != expectedPodName {
		t.Errorf("getPluginPod() returned the wrong pod. Expected: %s, Got: %s", expectedPodName, pod.Name)
	}
}
func TestGetPodStatusString(t *testing.T) {
	pod := &kcorev1.Pod{
		Status: kcorev1.PodStatus{
			Phase: kcorev1.PodRunning,
			Conditions: []kcorev1.PodCondition{
				{
					Type:   kcorev1.PodReady,
					Status: kcorev1.ConditionTrue,
				},
			},
		},
	}

	expectedStatus := "Running"
	status := getPodStatusString(pod)
	if status != expectedStatus {
		t.Errorf("getPodStatusString() returned the wrong status. Expected: %s, Got: %s", expectedStatus, status)
	}
}
