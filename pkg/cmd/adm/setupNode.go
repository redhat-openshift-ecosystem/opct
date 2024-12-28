package adm

import (
	"context"
	"fmt"

	"github.com/redhat-openshift-ecosystem/opct/pkg/client"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type setupNodeInput struct {
	nodeName string
	yes      bool
}

var setupNodeArgs setupNodeInput
var setupNodeCmd = &cobra.Command{
	Use:     "setup-node",
	Example: "opct adm setup-node",
	Short:   "Setup the node for the validation process.",
	Run:     setupNodeRun,
}

func init() {
	setupNodeCmd.Flags().BoolVarP(&setupNodeArgs.yes, "yes", "y", false, "Node to set required label and taints")
	setupNodeCmd.Flags().StringVar(&setupNodeArgs.nodeName, "node", "", "Node to set required label and taints")
}

func discoverNode(clientset kubernetes.Interface) (string, error) {
	// list all pods with label prometheus=k8s in namespace openshift-monitoring
	pods, err := clientset.CoreV1().Pods("openshift-monitoring").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "prometheus=k8s",
	})
	if err != nil {
		log.Fatalf("Failed to list Prometheus pods on namespace openshift-monitoring: %v", err)
	}

	// get the node running on those pods
	if len(pods.Items) < 1 {
		log.Fatalf("Expected at least 1 Prometheus pod, got %d. Use --name to manually set the node.", len(pods.Items))
	}
	nodesRunningPrometheus := map[string]struct{}{}
	for _, pod := range pods.Items {
		log.Infof("Prometheus pod %s is running on node %s, adding to skip list...", pod.Name, pod.Spec.NodeName)
		nodesRunningPrometheus[pod.Spec.NodeName] = struct{}{}
	}

	// list all nodes with label node-role.kubernetes.io/worker=''
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/worker=",
	})
	if err != nil {
		log.Fatalf("Failed to list nodes: %v", err)
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

func setupNodeRun(cmd *cobra.Command, args []string) {
	kclient, _, err := client.CreateClients()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	if setupNodeArgs.nodeName == "" {
		setupNodeArgs.nodeName, err = discoverNode(kclient)
		if err != nil {
			log.Fatalf("Failed to discover node: %v", err)
		}
	}
	log.Infof("Setting up node %s...", setupNodeArgs.nodeName)

	node, err := kclient.CoreV1().Nodes().Get(context.TODO(), setupNodeArgs.nodeName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Failed to get node %s: %v", setupNodeArgs.nodeName, err)
	}

	// Ask if the user wants to proceed with applying changes to the node
	if !setupNodeArgs.yes {
		fmt.Printf("Are you sure you want to apply changes to node %s? (y/n): ", setupNodeArgs.nodeName)
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil {
			log.Fatalf("Failed to read user response: %v", err)
		}
		if response != "y" && response != "Y" {
			fmt.Println("Aborted.")
			return
		}
	}

	// Create the labels map
	node.ObjectMeta.Labels["node-role.kubernetes.io/tests"] = ""
	node.Spec.Taints = append(node.Spec.Taints, v1.Taint{
		Key:    "node-role.kubernetes.io/tests",
		Value:  "",
		Effect: v1.TaintEffectNoSchedule,
	})
	// Update the node labels
	_, err = kclient.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		log.Fatalf("Failed to update node labels: %v", err)
	}
}
