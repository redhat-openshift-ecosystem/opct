package run

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	coclient "github.com/openshift/client-go/config/clientset/versioned"
	irclient "github.com/openshift/client-go/imageregistry/clientset/versioned"
	mcfgclientset "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	"github.com/redhat-openshift-ecosystem/opct/pkg"
	"github.com/redhat-openshift-ecosystem/opct/pkg/client"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

// PreRunValidations performs some validations before running the environment
func (r *RunOptions) PreRunValidations(kclient kubernetes.Interface) []error {
	log.Info("Starting preflight validations")
	allErrors := []error{}

	// Get ConfigV1 client for Cluster Operators
	restConfig, err := client.CreateRestConfig()
	if err != nil {
		log.Errorf("error creating rest config: %v", err)
		return []error{err}
	}

	// Validate if all configured container images are accessible
	errImages := validateContainerImagesAccessibility([]string{
		r.PluginsImage,
		r.OpenshiftTestsImage,
		r.CollectorImage,
		r.MustGatherMonitoringImage,
		pkg.ControllerImage,
	})
	if len(errImages) > 0 {
		log.Errorf("preflights checks failed: configured images are not accessible: %v", errImages)
		allErrors = append(allErrors, fmt.Errorf("configured images are not accessible: %v", errImages))
	}

	// Check if Cluster Operators are stable
	errCos := validateClusterOperators(r, restConfig)
	if len(errCos) > 0 {
		log.Errorf("preflights checks failed: operators are not in ready state, check the status with 'oc get clusteroperator': %v", errCos)
		allErrors = append(allErrors, fmt.Errorf("operators are not in ready state: %v", errCos))
	}

	// Validate if the image registry is in managed state
	errIr := validateImageRegistry(r, restConfig)
	if len(errIr) > 0 {
		log.Errorf("preflights checks failed: image registry is not in managed state: %v", errIr)
		allErrors = append(allErrors, fmt.Errorf("image registry is not in managed state: %v", errIr))
	}

	coreClient := kclient.CoreV1()

	// Check if dedicated node is set
	errDedicatedNode := validateDedicatedNode(r, coreClient)
	if len(errDedicatedNode) > 0 {
		log.Errorf("preflights checks failed: dedicated node is not set: %v", errDedicatedNode)
		allErrors = append(allErrors, fmt.Errorf("dedicated node is not set: %v", errDedicatedNode))
	}

	// Check if opct namespace already exists
	errOpctNamespace := validateOpctNamespace(r, coreClient)
	if len(errOpctNamespace) > 0 {
		log.Errorf("preflights checks failed: opct namespace already exists: %v", errOpctNamespace)
		allErrors = append(allErrors, fmt.Errorf("opct namespace already exists: %v", errOpctNamespace))
	}

	// Check MachineConfigPool when upgrade.
	errMachineConfigPool := validateMachineConfigPool(r, restConfig)
	if len(errMachineConfigPool) > 0 {
		log.Errorf("preflights checks failed: machine config pool is not in the expected state: %v", errMachineConfigPool)
		allErrors = append(allErrors, fmt.Errorf("machine config pool is not in the expected state: %v", errMachineConfigPool))
	}

	log.Info("Preflight validations completed")
	return allErrors
}

// checkPluginImages validates that all required container images are accessible
func validateContainerImagesAccessibility(images []string) []error {
	log.Debugf("Validating container images accessibility: %v", images)
	var result []error
	for _, image := range images {
		if image == "" {
			continue
		}

		if strings.HasPrefix(image, "image-registry.openshift-image-registry.svc:5000/") {
			log.Debugf("Skipping image accessibility check for %s", image)
			continue
		}

		log.Debugf("Validating image accessibility: %s", image)
		// Use oc image info to validate image exists and is accessible
		cmd := exec.Command("oc", "image", "info", image)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			errMsg := strings.TrimSpace(stderr.String())
			if errMsg == "" {
				errMsg = err.Error()
			}

			// If the error is about manifest list, the image exists and is valid
			if strings.Contains(errMsg, "manifest list") && strings.Contains(errMsg, "use --filter-by-os") {
				log.Debugf("Image %s is a valid manifest list", image)
			} else {
				// Image is not accessible
				result = append(result, fmt.Errorf("image %s is not accessible: %s", image, errMsg))
			}
		} else {
			log.Debugf("Image %s is accessible", image)
		}
	}

	return result
}

func validateClusterOperators(r *RunOptions, restConfig *rest.Config) []error {
	log.Debugf("Validating Cluster Operators stability")
	var result []error

	// Create OpenShift config client
	oc, err := coclient.NewForConfig(restConfig)
	if err != nil {
		return []error{err}
	}

	// Create context with timeout for validation
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(r.validationTimeout)*time.Second)
	defer cancel()

	// Wait for cluster operators with retry logic
	retryInterval := time.Duration(r.validationRetryInterval) * time.Second
	err = waitForClusterOperators(ctx, oc, retryInterval)

	if err != nil {
		log.Errorf("preflights checks failed: operators are not in ready state, check the status with 'oc get clusteroperator': %v", err)
		if r.devSkipChecks {
			log.Warnf("DEVEL MODE, THIS IS NOT SUPPORTED: Skipping Cluster Operator checks: %v", err)
		} else {
			result = append(result, fmt.Errorf("operators are not in ready state: %w", err))
		}
	}

	return result
}

// validateImageRegistry validates that the image registry is in managed state
func validateImageRegistry(r *RunOptions, restConfig *rest.Config) []error {
	log.Debugf("Validating Image Registry configuration")
	var result []error
	irClient, err := irclient.NewForConfig(restConfig)
	if err != nil {
		return []error{err}
	}
	managedRegistry, err := checkRegistry(irClient)
	if err != nil {
		return []error{err}
	}

	if !managedRegistry {
		log.Errorf("preflights checks failed: image registry is not in managed state")
		if r.devSkipChecks {
			log.Warnf("DEVEL MODE, THIS IS NOT SUPPORTED: Skipping unmanaged image registry check")
		} else {
			result = append(result, fmt.Errorf("image registry is not in managed state"))
		}
	}

	return result
}

// validateDedicatedNode validates that the dedicated node is set and has the required label and taints
func validateDedicatedNode(r *RunOptions, coreClient corev1.CoreV1Interface) []error {
	msgPrefix := "Validating Dedicated Node"
	log.Debug(msgPrefix)
	var result []error

	if !r.dedicated {
		return result
	}

	log.Debugf("%s: Checking if required node label and taints exists", msgPrefix)
	nodes, err := coreClient.Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: pkg.DedicatedNodeRoleLabelSelector,
	})
	if err != nil {
		return []error{fmt.Errorf("error getting the Node list: %w", err)}
	}
	nodeFound := len(nodes.Items) > 0
	if len(nodes.Items) == 0 {
		result = append(result, fmt.Errorf(`missing dedicated node. Set the label %q to a node and try again
Check the documentation[1] or run 'opct adm e2e-dedicated taint-node' to set the label and taints.
[1] https://redhat-openshift-ecosystem.github.io/provider-certification-tool/user/#standard-env-setup-node`, pkg.DedicatedNodeRoleLabelSelector))
	}
	if len(nodes.Items) > 2 {
		result = append(result, fmt.Errorf("too many nodes with label %q. Set the label to only one node and try again", pkg.DedicatedNodeRoleLabelSelector))
	}

	if nodeFound {
		node := nodes.Items[0]
		nodeTaintFound := false
		for _, taint := range node.Spec.Taints {
			if taint.Key == pkg.DedicatedNodeRoleLabel {
				nodeTaintFound = true
				break
			}
		}
		if !nodeTaintFound {
			result = append(result, fmt.Errorf("missing taint \"%s='':NoSchedule\" in the dedicated node %q. Set the taint and try again", pkg.DedicatedNodeRoleLabel, node.Name))
		}
	}

	return result
}

// validateOpctNamespace validates if the opct namespace not exists.
func validateOpctNamespace(r *RunOptions, coreClient corev1.CoreV1Interface) []error {
	checkMsgPrefix := "Validating OPCT namespace"
	log.Debug(checkMsgPrefix)
	var result []error

	p, err := coreClient.Namespaces().Get(context.TODO(), pkg.CertificationNamespace, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Debugf("%s: opct namespace not found, skipping validation", checkMsgPrefix)
			return result
		}
		return []error{fmt.Errorf("%s: error getting opct namespace: %w", checkMsgPrefix, err)}
	}
	if p.Name != "" {
		result = append(result, fmt.Errorf("%s: %s namespace already exists. You must run 'destroy' to clean the environment and try again", checkMsgPrefix, pkg.CertificationNamespace))
	}
	return result
}

// Check if MachineConfigPool exists when upgrade mode is set.:
// - node selectors: node-role.kubernetes.io/tests=”
// - paused: true
func validateMachineConfigPool(r *RunOptions, restConfig *rest.Config) []error {
	var result []error

	if r.mode != "upgrade" {
		return result
	}

	log.Debugf("Validating Machine Config Pool")

	mcpName := "opct"
	machineConfigClient, err := mcfgclientset.NewForConfig(restConfig)
	if err != nil {
		return []error{fmt.Errorf("error creating machine config client: %w", err)}
	}
	poolList, err := machineConfigClient.MachineconfigurationV1().MachineConfigPools().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return []error{fmt.Errorf("getting MachineConfigPools failed: %w", err)}
	}

	if len(poolList.Items) == 0 {
		return []error{fmt.Errorf("no MachineConfigPool objects found when listing MachineConfigPools")}
	}

	isFound := false
	isPaused := false
	for _, pool := range poolList.Items {
		if pool.Name == mcpName {
			isFound = true
			if !pool.Spec.Paused {
				result = append(result, fmt.Errorf("machineConfigPool %q is not paused", mcpName))
			}
			isPaused = true
		}
	}
	if !isFound {
		result = append(result, fmt.Errorf("machineConfigPool %q not found, create it and try again", mcpName))
		result = append(result, fmt.Errorf(`MachineConfigPool not found, create it with the following instructions: $ cat << EOF  | oc apply -f -
---
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfigPool
metadata:
  name: opct
spec:
  machineConfigSelector:
    matchExpressions:
      - key: machineconfiguration.openshift.io/role
        operator: In
        values: [worker,opct]
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/tests: ""
  paused: true
EOF`))

	}
	if !isPaused {
		result = append(result, fmt.Errorf("machineConfigPool %q is not paused, set `spec.pause=true` and try again", mcpName))
	}

	return result
}
