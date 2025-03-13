package e2ed

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	types "github.com/redhat-openshift-ecosystem/opct/pkg"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

// newCmdE2eDedicatedController returns a new cobra.Command for starting the e2e-dedicated controller.
// The controller watches all pods failing to schedule due to dedicated node configuration, specifically
// for OPCT, which requires a toleration configuration in the pod spec. Failed pods will be mutated to
// use the required toleration, preventing false-positive failures.
func newCmdE2eDedicatedController() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Start the e2e-dedicated controller.",
		Long: `Start the e2e-dedicated controller to watch all pods failing to schedule due the
		dedicated node configuration, specifically for OPCT, which requires a toleration configuration
		in the pod spec.
		Failed pods will be mutated to use the required toleration, preventing false-positive failures.`,
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		controllerRun()
	}

	return cmd
}

type podMutateStatus struct {
	Successed uint64
	Failed    uint64
	Skipped   uint64
}

var (
	podCounters      = make(map[string]*podMutateStatus)
	podCountersMutex sync.Mutex
)

// getPodKey constructs a unique key for a pod using its namespace and name.
func getPodKey(namespace, podName string) string {
	return fmt.Sprintf("%s/%s", namespace, podName)
}

// ensureCounter checks if a counter exists for the given pod key and initializes it if not.
func ensureCounter(podKey string) {
	podCountersMutex.Lock()
	defer podCountersMutex.Unlock()
	if _, ok := podCounters[podKey]; !ok {
		podCounters[podKey] = &podMutateStatus{}
	}
}

func showCounter(podKey string) {
	pk := podCounters[podKey]
	log.Debugf("Metrics counter for %s: success(%d) skipped(%d) failed(%d)", podKey, pk.Successed, pk.Skipped, pk.Failed)
}

// incCounterFailure increments the failure counter for the given pod key.
func incCounterFailure(podKey string) {
	ensureCounter(podKey)
	podCountersMutex.Lock()
	podCounters[podKey].Failed += 1
	podCountersMutex.Unlock()
	showCounter(podKey)
}

// incCounterSkipped increments the skipped counter for the given pod key.
func incCounterSkipped(podKey string) {
	ensureCounter(podKey)
	podCountersMutex.Lock()
	podCounters[podKey].Skipped += 1
	podCountersMutex.Unlock()
	showCounter(podKey)
}

// incCounterSuccess increments the success counter for the given pod key.
func incCounterSuccess(podKey string) {
	ensureCounter(podKey)
	podCountersMutex.Lock()
	podCounters[podKey].Successed += 1
	podCountersMutex.Unlock()
	showCounter(podKey)
}

// showCounters prints the current counters for all pods.
func showCounters(stop chan struct{}) {
	previousTotal := 0
	backoff := 10 * time.Second
	for {
		select {
		case <-stop:
			return
		default:
			// Show summary of total pods changed for this controller.
			failed := 0
			skipped := 0
			successed := 0
			for podKey := range podCounters {
				failed += int(podCounters[podKey].Failed)
				skipped += int(podCounters[podKey].Skipped)
				successed += int(podCounters[podKey].Successed)
			}
			// show summary with backoff to prevent many messages.
			if previousTotal != len(podCounters) {
				log.Printf("Metrics summary: Total pods changed: %d. successed(%d) skipped(%d) failed(%d)", len(podCounters), successed, skipped, failed)
				previousTotal = len(podCounters)
				time.Sleep(1 * time.Second)
				continue
			}
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 2*time.Minute {
				backoff = 2 * time.Minute
				log.Printf("No change in metrics in the last 2 minutes.")
			}
		}

	}
}

// controllerRun starts the dedicated-e2e controller which watches for pods failing to schedule
// in the e2e namespaces and mutates them to include the required toleration.
func controllerRun() {
	log.Info("Starting the e2e-dedicated controller...")

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get in-cluster config: %v. Ensure the KUBECONFIG environment variable is set or config is in the path:\n", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v\n", err)
	}

	log.Info("Creating the informer to watch pods failed to schedule in e2e namespaces...")

	// Create the informer to watch pods every one second, then mutate when updated. The mutation
	// will add the required toleration.
	_, controller := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: cache.NewListWatchFromClient(
			clientset.CoreV1().RESTClient(),
			"pods",
			corev1.NamespaceAll,
			fields.Everything(),
		),
		ObjectType: &corev1.Pod{},
		Handler: cache.ResourceEventHandlerDetailedFuncs{
			UpdateFunc: func(oldObj, newObj interface{}) {
				newPod := newObj.(*corev1.Pod)
				// ensure only e2e and opct (for tests) will be changed
				if strings.HasPrefix(newPod.Namespace, "e2e-") || strings.HasPrefix(newPod.Namespace, "opct") {
					for _, condition := range newPod.Status.Conditions {
						// act only when the pod failed to schedule due the opct environment: one random worker node has taints preventing scheduling the node.
						if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse && condition.Reason == corev1.PodReasonUnschedulable {
							handleFailedScheduling(clientset, newPod)
						}
					}
				}
			},
		},
		ResyncPeriod: 1 * time.Second,
	})

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)
	go showCounters(stop)

	select {}
}

// handleFailedScheduling is the update informer function handler to mutate the pod object adding the
// required tolerations, when not exists. Informer function handlers can't return errors,
// when operation fails, it will be logged in the default log handler.
func handleFailedScheduling(clientset *kubernetes.Clientset, pod *corev1.Pod) {
	podKey := getPodKey(pod.Namespace, pod.Name)
	skipped := false
	log.Debugf("[%s] starting the handler for failed scheduling pods", podKey)

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pod, err := clientset.CoreV1().Pods(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
		if err != nil {
			// Update functions can't return errors
			log.Errorf("[%s] failed to get the pod by name: %v", podKey, err)
			return nil
		}

		// add tolerations only if not yet applied to the node.
		hasToleration := false
		for _, toleration := range pod.Spec.Tolerations {
			if toleration.Key == types.DedicatedNodeRoleLabel {
				hasToleration = true
				break
			}
		}

		if hasToleration {
			log.Debugf("[%s] skipping pod already has the required toleration", podKey)
			incCounterSkipped(podKey)
			skipped = true
			return nil
		}
		toleration := corev1.Toleration{
			Key:      types.DedicatedNodeRoleLabel,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		}
		pod.Spec.Tolerations = append(pod.Spec.Tolerations, toleration)

		_, updateErr := clientset.CoreV1().Pods(pod.Namespace).Update(context.Background(), pod, metav1.UpdateOptions{})
		if updateErr != nil {
			log.Errorf("[%s] failed to update pod: %v", podKey, updateErr)
		}
		return updateErr
	})

	if retryErr != nil {
		log.Errorf("[%s] failed to update pod: %v", podKey, retryErr)
		incCounterFailure(podKey)
	} else if !skipped {
		log.Infof("[%s] successfully added toleration to pod", podKey)
		incCounterSuccess(podKey)
	}
}
