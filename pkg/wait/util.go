package wait

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"

	"github.com/redhat-openshift-ecosystem/opct/pkg"
)

const (
	aggregatorLabelSelector = "component=sonobuoy,sonobuoy-component=aggregator"
	aggregatorReadyTimeout  = 3 * time.Minute
	retryLogInterval        = 30 * time.Second
)

// WaitForRequiredResources will wait for the sonobuoy pod in the sonobuoy namespace to go into
// a Running/Ready state and then return nil.
func WaitForRequiredResources(kclient kubernetes.Interface) error {
	var obj kruntime.Object

	restClient := kclient.CoreV1().RESTClient()

	lw := cache.NewFilteredListWatchFromClient(restClient, "pods", pkg.CertificationNamespace, func(options *metav1.ListOptions) {
		options.LabelSelector = aggregatorLabelSelector
	})

	// Wait for Sonobuoy aggregator pod to become Ready
	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute*10)
	defer cancel()
	_, err := watchtools.UntilWithSync(ctx, lw, obj, nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Error:
			return false, fmt.Errorf("error waiting for sonobuoy to start: %w", event.Object.(error))
		case watch.Deleted:
			return false, errors.New("sonobuoy pod deleted while waiting to become ready")
		}

		pod, isPod := event.Object.(*v1.Pod)
		if !isPod {
			return false, errors.New("type error watching for sononbuoy to start")
		}

		if pod.Status.Phase == v1.PodRunning && podIsReady(pod) {
			return true, nil
		}

		// Loop again
		return false, nil
	})
	if err != nil {
		return err
	}

	return nil
}

// WaitForAggregatorReady polls until the sonobuoy aggregator pod is Running/Ready.
// Uses List (no watch/reflector) with exponential backoff for transient API errors.
func WaitForAggregatorReady(parentCtx context.Context, kclient kubernetes.Interface) error {
	backoff := wait.Backoff{
		Duration: 2 * time.Second,
		Factor:   2.0,
		Cap:      30 * time.Second,
		Steps:    30,
	}
	return waitForAggregatorReady(parentCtx, kclient, backoff, aggregatorReadyTimeout)
}

// waitForAggregatorReady drives the polling loop directly instead of using
// wait.ExponentialBackoffWithContext, which stops and returns ErrWaitTimeout as
// soon as the first Cap-truncated sleep occurs (i.e. it treats reaching the cap
// as exhaustion). That would end polling after ~30s here instead of honoring
// the full timeout. Looping manually and bounding only on ctx.Done() avoids
// that, while still reusing backoff.Step() for the growing sleep interval.
//
// It also distinguishes a canceled/expired parentCtx from the timeout applied
// here, so callers get the real cancellation reason instead of a misleading
// "aggregator not ready" timeout error.
func waitForAggregatorReady(parentCtx context.Context, kclient kubernetes.Interface, backoff wait.Backoff, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	startedAt := time.Now()
	var lastWarnAt time.Time
	warnIfDue := func(msg string, err error) {
		now := time.Now()
		if now.Sub(startedAt) < retryLogInterval {
			return
		}
		if !lastWarnAt.IsZero() && now.Sub(lastWarnAt) < retryLogInterval {
			return
		}
		lastWarnAt = now
		if err != nil {
			log.WithError(err).Warn(msg)
			return
		}
		log.Warn(msg)
	}

	for {
		pods, err := kclient.CoreV1().Pods(pkg.CertificationNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: aggregatorLabelSelector,
		})
		switch {
		case err != nil:
			if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || apierrors.IsBadRequest(err) {
				return fmt.Errorf("listing sonobuoy aggregator pods: %w", err)
			}
			warnIfDue("error listing sonobuoy aggregator pods, retrying", err)
		case len(pods.Items) == 0:
			warnIfDue("sonobuoy aggregator pod not found yet, retrying", nil)
		default:
			ready := false
			for i := range pods.Items {
				pod := &pods.Items[i]
				if pod.Status.Phase == v1.PodRunning && podIsReady(pod) {
					ready = true
					break
				}
			}
			if ready {
				return nil
			}
			warnIfDue("sonobuoy aggregator pod is not ready yet, retrying", nil)
		}

		select {
		case <-ctx.Done():
			if parentCtx.Err() != nil {
				return fmt.Errorf("waiting for sonobuoy aggregator pod: %w", parentCtx.Err())
			}
			return fmt.Errorf("timeout waiting for sonobuoy aggregator pod to become ready after %s: %w", timeout, ctx.Err())
		case <-time.After(backoff.Step()):
		}
	}
}

func podIsReady(pod *v1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}
