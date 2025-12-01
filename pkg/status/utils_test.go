package status

import (
	"testing"

	kcorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func TestGetPodEvents(t *testing.T) {
	tests := []struct {
		name          string
		events        []kcorev1.Event
		expectedCount int
		expectedType  string
	}{
		{
			name: "warning events only",
			events: []kcorev1.Event{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-event-1",
						Namespace: "test-namespace",
					},
					InvolvedObject: kcorev1.ObjectReference{
						Name: "test-pod",
						Kind: "Pod",
					},
					Type:    kcorev1.EventTypeWarning,
					Reason:  "BackOff",
					Message: "Back-off pulling image",
				},
			},
			expectedCount: 1,
			expectedType:  kcorev1.EventTypeWarning,
		},
		{
			name: "failed events",
			events: []kcorev1.Event{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-event-2",
						Namespace: "test-namespace",
					},
					InvolvedObject: kcorev1.ObjectReference{
						Name: "test-pod",
						Kind: "Pod",
					},
					Type:    kcorev1.EventTypeWarning,
					Reason:  "Failed",
					Message: "Failed to pull image",
				},
			},
			expectedCount: 1,
			expectedType:  kcorev1.EventTypeWarning,
		},
		{
			name: "mixed events - only errors returned",
			events: []kcorev1.Event{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-event-normal",
						Namespace: "test-namespace",
					},
					InvolvedObject: kcorev1.ObjectReference{
						Name: "test-pod",
						Kind: "Pod",
					},
					Type:    kcorev1.EventTypeNormal,
					Reason:  "Started",
					Message: "Container started",
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-event-warning",
						Namespace: "test-namespace",
					},
					InvolvedObject: kcorev1.ObjectReference{
						Name: "test-pod",
						Kind: "Pod",
					},
					Type:    kcorev1.EventTypeWarning,
					Reason:  "BackOff",
					Message: "Back-off pulling image",
				},
			},
			expectedCount: 1,
			expectedType:  kcorev1.EventTypeWarning,
		},
		{
			name:          "no error events",
			events:        []kcorev1.Event{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset with events
			eventObjects := make([]runtime.Object, len(tt.events))
			for i := range tt.events {
				eventObjects[i] = &tt.events[i]
			}
			kclient := fake.NewSimpleClientset(eventObjects...)

			events, err := getPodEvents(kclient, "test-namespace", "test-pod")
			if err != nil {
				t.Errorf("getPodEvents() returned an error: %v", err)
			}

			if len(events) != tt.expectedCount {
				t.Errorf("getPodEvents() returned wrong number of events. Expected: %d, Got: %d", tt.expectedCount, len(events))
			}

			if tt.expectedCount > 0 && events[0].Type != tt.expectedType {
				t.Errorf("getPodEvents() returned wrong event type. Expected: %s, Got: %s", tt.expectedType, events[0].Type)
			}
		})
	}
}

func TestGetPodEventsMessage(t *testing.T) {
	tests := []struct {
		name            string
		events          []kcorev1.Event
		expectedMessage string
	}{
		{
			name: "ImagePullBackOff error",
			events: []kcorev1.Event{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-event-1",
						Namespace: "test-namespace",
					},
					InvolvedObject: kcorev1.ObjectReference{
						Name: "test-pod",
						Kind: "Pod",
					},
					Type:    kcorev1.EventTypeWarning,
					Reason:  "BackOff",
					Message: "Back-off pulling image \"quay.io/expired:image\"",
				},
			},
			expectedMessage: "BackOff: Back-off pulling image \"quay.io/expired:image\"",
		},
		{
			name: "Failed to pull image",
			events: []kcorev1.Event{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-event-2",
						Namespace: "test-namespace",
					},
					InvolvedObject: kcorev1.ObjectReference{
						Name: "test-pod",
						Kind: "Pod",
					},
					Type:    kcorev1.EventTypeWarning,
					Reason:  "Failed",
					Message: "Failed to pull image: manifest unknown",
				},
			},
			expectedMessage: "Failed: Failed to pull image: manifest unknown",
		},
		{
			name:            "no events",
			events:          []kcorev1.Event{},
			expectedMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset with events
			eventObjects := make([]runtime.Object, len(tt.events))
			for i := range tt.events {
				eventObjects[i] = &tt.events[i]
			}
			kclient := fake.NewSimpleClientset(eventObjects...)

			message := getPodEventsMessage(kclient, "test-namespace", "test-pod")
			if message != tt.expectedMessage {
				t.Errorf("getPodEventsMessage() returned wrong message. Expected: %s, Got: %s", tt.expectedMessage, message)
			}
		})
	}
}
