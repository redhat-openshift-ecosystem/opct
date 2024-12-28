package wait

import (
	"context"
	"errors"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"

	"github.com/redhat-openshift-ecosystem/opct/pkg"
)

// WaitForRequiredResources will wait for the sonobuoy pod in the sonobuoy namespace to go into
// a Running/Ready state and then return nil.
func WaitForRequiredResources(kclient kubernetes.Interface) error {
	var obj kruntime.Object

	restClient := kclient.CoreV1().RESTClient()

	lw := cache.NewFilteredListWatchFromClient(restClient, "pods", pkg.CertificationNamespace, func(options *metav1.ListOptions) {
		options.LabelSelector = "component=sonobuoy,sonobuoy-component=aggregator"
	})

	// Wait for Sonobuoy Pods to become Ready
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

func podIsReady(pod *v1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}
