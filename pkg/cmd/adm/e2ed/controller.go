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
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
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

// dedicatedNodeNames returns the names of the nodes holding the OPCT dedicated node taint.
// Only the NoSchedule effect applied by "opct adm e2e-dedicated taint-node" is considered, since
// that is the only effect the injected toleration matches.
func dedicatedNodeNames(nodes []*corev1.Node) map[string]struct{} {
	dedicated := make(map[string]struct{})
	for _, node := range nodes {
		for _, taint := range node.Spec.Taints {
			if taint.Key == types.DedicatedNodeRoleLabel && taint.Effect == corev1.TaintEffectNoSchedule {
				dedicated[node.Name] = struct{}{}
				break
			}
		}
	}
	return dedicated
}

// isPinnedToDedicatedNode reports whether the pod can only ever run on the OPCT dedicated node,
// which is the single case where the dedicated node taint is the reason the pod can't be scheduled.
//
// The scheduling failure message can't be used to make this decision: the scheduler collapses the
// taint reasons into a generic "N node(s) had untolerated taint(s)" whenever more than one taint key
// is involved, which is always the case in OpenShift because the control plane nodes are tainted too.
//
// Pods that are merely looking for free capacity are left alone, so the cluster behaves like a
// regular cluster with one less worker instead of silently lending the reserved node to them.
func isPinnedToDedicatedNode(pod *corev1.Pod, nodes []*corev1.Node) bool {
	dedicated := dedicatedNodeNames(nodes)
	if len(dedicated) == 0 {
		return false
	}

	// Pods bound to a node by name bypass the node selector and affinity terms.
	if pod.Spec.NodeName != "" {
		_, ok := dedicated[pod.Spec.NodeName]
		return ok
	}

	// GetRequiredNodeAffinity covers both spec.nodeSelector and the required node affinity terms.
	// A pod without any of them matches every node, so it is not pinned.
	required := nodeaffinity.GetRequiredNodeAffinity(pod)
	matched := false
	for _, node := range nodes {
		ok, err := required.Match(node)
		if err != nil {
			// Skipping the node would drop the only candidate able to prove the pod is not pinned,
			// so an unresolvable placement leaves the pod untouched instead of mutating it.
			log.Debugf("[%s] failed to match node %s against the pod node affinity: %v", getPodKey(pod.Namespace, pod.Name), node.Name, err)
			return false
		}
		if !ok {
			continue
		}
		if _, isDedicated := dedicated[node.Name]; !isDedicated {
			return false
		}
		matched = true
	}
	return matched
}

// listNodes returns the nodes currently held by the informer store.
func listNodes(store cache.Store) []*corev1.Node {
	nodes := []*corev1.Node{}
	for _, obj := range store.List() {
		if node, ok := obj.(*corev1.Node); ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// controllerRun starts the dedicated-e2e controller which watches for pods failing to schedule
// in the e2e namespaces and mutates them to include the required toleration.
func controllerRun() {
	log.Info("Starting the e2e-dedicated controller...")

	// The controller normally runs in-cluster, the kubeconfig lookup allows running it locally
	// against a live cluster while developing or reproducing scheduling issues. The in-cluster
	// config is resolved first so a kubeconfig reaching the pod can't shadow the service account.
	config, err := func() (*rest.Config, error) {
		if cfg, err := rest.InClusterConfig(); err == nil {
			return cfg, nil
		}
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
		return kubeConfig.ClientConfig()
	}()
	if err != nil {
		log.Fatalf("Failed to get the cluster config: %v. Ensure the KUBECONFIG environment variable is set or config is in the path:\n", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v\n", err)
	}

	stop := make(chan struct{})
	defer close(stop)

	log.Info("Creating the informer to watch the nodes holding the dedicated node taint...")

	// The node cache is used to decide whether a pod is pinned to the dedicated node, so it must be
	// populated before the pod informer starts handling events.
	nodeStore, nodeController := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: cache.NewListWatchFromClient(
			clientset.CoreV1().RESTClient(),
			"nodes",
			corev1.NamespaceAll,
			fields.Everything(),
		),
		ObjectType:   &corev1.Node{},
		Handler:      cache.ResourceEventHandlerDetailedFuncs{},
		ResyncPeriod: 30 * time.Second,
	})
	go nodeController.Run(stop)
	if !cache.WaitForCacheSync(stop, nodeController.HasSynced) {
		log.Fatal("Failed to sync the node cache")
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
							// only pods that can't run anywhere but the dedicated node are false-positives
							// caused by the opct environment. Pods failing to schedule for any other reason,
							// like resource exhaustion, must be left untouched: mutating them lets them escape
							// to the reserved node and bumps metadata.generation, breaking upstream tests.
							if !isPinnedToDedicatedNode(newPod, listNodes(nodeStore)) {
								log.Debugf("[%s] skipping pod not pinned to the dedicated node", getPodKey(newPod.Namespace, newPod.Name))
								continue
							}
							handleFailedScheduling(clientset, newPod)
						}
					}
				}
			},
		},
		ResyncPeriod: 1 * time.Second,
	})

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
