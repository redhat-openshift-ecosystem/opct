package e2ed

import (
	"context"
	"fmt"

	types "github.com/redhat-openshift-ecosystem/opct/pkg"

	"github.com/redhat-openshift-ecosystem/opct/pkg/client"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const computeNodeLabelSelector = "node-role.kubernetes.io/worker="
const monitoringNamespace string = "openshift-monitoring"
const monitoringPrometheusLabelSelector string = "prometheus=k8s"

type taintNodeInput struct {
	nodeName string
	yes      bool
}

var taintNodeArgs taintNodeInput
var taintNodeCmd = &cobra.Command{
	Use:     "taint-node",
	Example: "opct adm e2e-dedicated taint-node [options]",
	Short:   "Setup the 'dedicated' node for the validation environment.",
	Long: `Setup the 'dedicated' nodes to the test environment used by OPCT.
The 'dedicated' node is used to host all the service and jobs used during the workflow
execution. It is used to prevent disruption or evictions in the test environment.
The command 'taint-node' automatically discovers a node which is not hosting the
monitoring services, such as Prometheus pods, setting up quickly the test environment
without forcing the controllers to rebalance.

You can automatically accept the selected node with the option --yes.

Alternatively you can set the custom node with the option --node <node-name>.`,
	Run: taintNodeRun,
}

func init() {
	taintNodeCmd.Flags().BoolVarP(&taintNodeArgs.yes, "yes", "y", false, "Force to apply the changes without survey. Default: false")
	taintNodeCmd.Flags().StringVar(&taintNodeArgs.nodeName, "node", "", "Use the node name to set required label and taints")
}

func discoverNode(clientset kubernetes.Interface) (string, error) {
	// list all pods with label prometheus=k8s in namespace openshift-monitoring
	pods, err := clientset.CoreV1().Pods(monitoringNamespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: monitoringPrometheusLabelSelector,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list Prometheus pods in namespace %s: %v", monitoringNamespace, err)
	}

	// get the node running on those pods
	if len(pods.Items) < 1 {
		return "", fmt.Errorf("expected at least 1 Prometheus pod, got %d. Use --node to manually set the node", len(pods.Items))
	}
	nodesRunningPrometheus := map[string]struct{}{}
	for _, pod := range pods.Items {
		log.Infof("Prometheus pod %s is running on node %s, adding to skip list...", pod.Name, pod.Spec.NodeName)
		nodesRunningPrometheus[pod.Spec.NodeName] = struct{}{}
	}

	// list all nodes with label node-role.kubernetes.io/worker=''
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: computeNodeLabelSelector,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %v", err)
	}
	for _, node := range nodes.Items {
		if _, ok := nodesRunningPrometheus[node.Name]; !ok {
			return node.Name, nil
		}
	}
	forceNode := nodes.Items[0].Name
	log.Warnf("No node available to run the validation process, using %s", forceNode)
	return forceNode, nil
}

func confirmAction(message string) bool {
	fmt.Printf("%s (y/n): ", message)
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		log.Fatalf("Failed to read user response: %v", err)
	}
	return response == "y" || response == "Y"
}

func applyTaintToNode(kclient kubernetes.Interface, nodeName string) error {
	log.Infof("Applying taint to node %s...", nodeName)
	node, err := kclient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %v", nodeName, err)
	}

	node.ObjectMeta.Labels[types.DedicatedNodeRoleLabel] = ""
	node.Spec.Taints = append(node.Spec.Taints, v1.Taint{
		Key:    types.DedicatedNodeRoleLabel,
		Value:  "",
		Effect: v1.TaintEffectNoSchedule,
	})

	_, err = kclient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update node label and taint: %v", err)
	}

	log.Infof("Successfully applied taint to node %s", nodeName)
	return nil
}

func taintNodeRun(cmd *cobra.Command, args []string) {
	kclient, _, err := client.CreateClients()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	if taintNodeArgs.nodeName == "" {
		taintNodeArgs.nodeName, err = discoverNode(kclient)
		if err != nil {
			log.Fatalf("Failed to discover node: %v", err)
		}
	}
	log.Infof("Setting up node %s...", taintNodeArgs.nodeName)

	if !taintNodeArgs.yes && !confirmAction(fmt.Sprintf("Are you sure you want to apply changes to node %s?", taintNodeArgs.nodeName)) {
		fmt.Println("Aborted.")
		return
	}

	if err := applyTaintToNode(kclient, taintNodeArgs.nodeName); err != nil {
		log.Fatalf("Failed to apply taint to node: %v", err)
	}
	log.Infof("Successfully applied taint to node %s", taintNodeArgs.nodeName)
}
