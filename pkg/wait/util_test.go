package wait

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/redhat-openshift-ecosystem/opct/pkg"
)

func readyAggregatorPod(name string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pkg.CertificationNamespace,
			Labels: map[string]string{
				"component":          "sonobuoy",
				"sonobuoy-component": "aggregator",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			Conditions: []v1.PodCondition{
				{Type: v1.PodReady, Status: v1.ConditionTrue},
			},
		},
	}
}

func pendingAggregatorPod(name string) *v1.Pod {
	pod := readyAggregatorPod(name)
	pod.Status.Phase = v1.PodPending
	pod.Status.Conditions = nil
	return pod
}

func TestWaitForAggregatorReady_ready(t *testing.T) {
	kclient := fake.NewSimpleClientset(readyAggregatorPod("sonobuoy-aggregator"))

	if err := WaitForAggregatorReady(context.Background(), kclient); err != nil {
		t.Fatalf("WaitForAggregatorReady() error = %v, want nil", err)
	}
}

func TestWaitForAggregatorReady_noPodsTimesOut(t *testing.T) {
	kclient := fake.NewSimpleClientset()

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	err := WaitForAggregatorReady(ctx, kclient)
	if err == nil {
		t.Fatal("WaitForAggregatorReady() error = nil, want timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitForAggregatorReady() error = %v, want deadline exceeded", err)
	}
}

func TestWaitForAggregatorReady_pendingTimesOut(t *testing.T) {
	kclient := fake.NewSimpleClientset(pendingAggregatorPod("sonobuoy-aggregator"))

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	err := WaitForAggregatorReady(ctx, kclient)
	if err == nil {
		t.Fatal("WaitForAggregatorReady() error = nil, want timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitForAggregatorReady() error = %v, want deadline exceeded", err)
	}
}

func TestWaitForAggregatorReady_forbidden(t *testing.T) {
	kclient := fake.NewSimpleClientset()
	kclient.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("denied"))
	})

	err := WaitForAggregatorReady(context.Background(), kclient)
	if err == nil {
		t.Fatal("WaitForAggregatorReady() error = nil, want forbidden error")
	}
	if !apierrors.IsForbidden(err) {
		t.Fatalf("WaitForAggregatorReady() error = %v, want forbidden error", err)
	}
}

// TestWaitForAggregatorReady_parentCancellation ensures a canceled parent
// context is reported as the real cancellation, not mislabeled as the
// aggregator readiness timeout applied internally.
func TestWaitForAggregatorReady_parentCancellation(t *testing.T) {
	kclient := fake.NewSimpleClientset(pendingAggregatorPod("sonobuoy-aggregator"))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := WaitForAggregatorReady(ctx, kclient)
	if err == nil {
		t.Fatal("WaitForAggregatorReady() error = nil, want cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitForAggregatorReady() error = %v, want context.Canceled", err)
	}
	if strings.Contains(err.Error(), "timeout waiting for sonobuoy aggregator pod to become ready after") {
		t.Fatalf("WaitForAggregatorReady() error = %v, parent cancellation must not be reported as an aggregator readiness timeout", err)
	}
}

// TestWaitForAggregatorReady_pollsUntilConfiguredTimeout ensures polling
// continues for the full configured timeout instead of stopping early once
// the backoff delay first hits its Cap (a quirk of
// wait.ExponentialBackoffWithContext, which treats a Cap-truncated sleep as
// exhaustion). A fake reactor only reports the pod ready after more List
// calls than the backoff's growth phase (Duration->Cap) would allow.
func TestWaitForAggregatorReady_pollsUntilConfiguredTimeout(t *testing.T) {
	kclient := fake.NewSimpleClientset()

	const callsBeforeReady = 5
	calls := 0
	kclient.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		calls++
		pod := pendingAggregatorPod("sonobuoy-aggregator")
		if calls >= callsBeforeReady {
			pod = readyAggregatorPod("sonobuoy-aggregator")
		}
		return true, &v1.PodList{Items: []v1.Pod{*pod}}, nil
	})

	// Duration grows past Cap after 3 steps (5ms, 10ms, 20ms), which would
	// exhaust wait.ExponentialBackoffWithContext well before callsBeforeReady
	// is reached. A generous timeout proves polling continues past that point.
	backoff := utilwait.Backoff{Duration: 5 * time.Millisecond, Factor: 2, Cap: 20 * time.Millisecond, Steps: 10}
	err := waitForAggregatorReady(context.Background(), kclient, backoff, time.Second)
	if err != nil {
		t.Fatalf("waitForAggregatorReady() error = %v, want nil once the pod becomes ready", err)
	}
	if calls < callsBeforeReady {
		t.Fatalf("expected at least %d list calls to exercise the cap-exhaustion path, got %d", callsBeforeReady, calls)
	}
}
