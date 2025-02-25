package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	coclient "github.com/openshift/client-go/config/clientset/versioned"
	irclient "github.com/openshift/client-go/imageregistry/clientset/versioned"
	mcfgclientset "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	"github.com/pkg/errors"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/version"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vmware-tanzu/sonobuoy/pkg/buildinfo"
	sonobuoyclient "github.com/vmware-tanzu/sonobuoy/pkg/client"
	"github.com/vmware-tanzu/sonobuoy/pkg/config"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/loader"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/manifest"
	v1 "k8s.io/api/core/v1"

	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/client"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/status"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/wait"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

type RunOptions struct {
	plugins *[]string

	sonobuoyImage   string
	imageRepository string

	// PluginsImage
	// defines the image containing plugins associated with the provider-certification-tool.
	// this variable is referenced by plugin manifest templates to dynamically reference the plugins image.
	PluginsImage              string
	CollectorImage            string
	MustGatherMonitoringImage string
	OpenshiftTestsImage       string

	timeout      int
	watch        bool
	mode         string
	upgradeImage string

	// devel flags
	devCount      string
	devSkipChecks bool

	// Dedicated node
	dedicated bool
}

const (
	defaultRunTimeoutSeconds = 21600
	defaultRunMode           = "regular"
	defaultUpgradeImage      = ""
	defaultDedicatedFlag     = true
	defaultRunWatchFlag      = false
)

func newRunOptions() *RunOptions {
	return &RunOptions{
		plugins: &[]string{},
	}
}

func hideOptionalFlags(cmd *cobra.Command, flag string) {
	err := cmd.Flags().MarkHidden(flag)
	if err != nil {
		log.Debugf("Unable to hide flag %s: %v", flag, err)
	}
}

func NewCmdRun() *cobra.Command {
	var err error
	var kclient kubernetes.Interface
	var sclient sonobuoyclient.Interface
	o := newRunOptions()

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the suite of tests for provider validation",
		Long:  `Launches the provider validation environment inside of an already running OpenShift cluster`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Client setup
			kclient, sclient, err = client.CreateClients()
			if err != nil {
				log.WithError(err).Error("pre-run failed when creating clients")
				return err
			}

			// Pre-checks and setup
			if err = o.PreRunCheck(kclient); err != nil {
				log.WithError(err).Error("pre-run failed when checking dependencies")
				return err
			}

			if err = o.PreRunSetup(kclient); err != nil {
				log.WithError(err).Error("pre-run failed when initializing the environment")
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("Running OPCT...")
			if err := o.Run(kclient, sclient); err != nil {
				log.WithError(err).Errorf("execution finished with errors.")
				return err
			}

			log.Info("Jobs scheduled! Waiting for resources be created...")
			if err := wait.WaitForRequiredResources(kclient); err != nil {
				log.WithError(err).Errorf("error waiting for required pods to become ready")
				return err
			}

			// Retrieve the first status and print it, finishing when --watch is not set.
			s := status.NewStatus(&status.StatusInput{
				Watch:   o.watch,
				KClient: kclient,
				SClient: sclient,
			})
			if err := s.WaitForStatusReport(cmd.Context()); err != nil {
				log.WithError(err).Error("error retrieving aggregator status")
				return err
			}

			if err := s.Update(); err != nil {
				log.WithError(err).Error("error retrieving update")
				return err
			}

			if err := s.Print(cmd); err != nil {
				log.WithError(err).Error("error showing status")
				return err
			}

			if !o.watch {
				log.Info("Sonobuoy pods are ready!")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&o.mode, "mode", defaultRunMode, "Run mode: Availble: regular, upgrade")
	cmd.Flags().StringVar(&o.upgradeImage, "upgrade-to-image", defaultUpgradeImage, "Target OpenShift Release Image. Example: oc adm release info 4.11.18 -o jsonpath={.image}")
	cmd.Flags().StringVar(&o.imageRepository, "image-repository", "", "Image repository containing required images test environment. Example: openshift-provider-cert-tool --mirror-repository mirror.repository.net/ocp-cert")

	cmd.Flags().IntVar(&o.timeout, "timeout", defaultRunTimeoutSeconds, "Execution timeout in seconds")
	cmd.Flags().BoolVarP(&o.watch, "watch", "w", defaultRunWatchFlag, "Keep watch status after running")

	cmd.Flags().StringVar(&o.devCount, "devel-limit-tests", "0", "Developer Mode only: run small random set of tests. Default: 0 (disabled)")
	cmd.Flags().BoolVar(&o.devSkipChecks, "devel-skip-checks", false, "Developer Mode only: skip checks")

	// Override build-int images use by plugins/steps in the standard workflow.
	cmd.Flags().StringVar(&o.sonobuoyImage, "sonobuoy-image", pkg.GetSonobuoyImage(), "Image override for the Sonobuoy worker and aggregator")
	cmd.Flags().StringVar(&o.PluginsImage, "plugins-image", pkg.GetPluginsImage(), "Image containing plugins to be executed.")
	cmd.Flags().StringVar(&o.CollectorImage, "collector-image", pkg.GetCollectorImage(), "Image containing the collector plugin.")
	cmd.Flags().StringVar(&o.MustGatherMonitoringImage, "must-gather-monitoring-image", pkg.GetMustGatherMonitoring(), "Image containing the must-gather monitoring plugin.")

	// devel can be override by quay.io/opct/openshift-tests:devel
	// opct run --devel-skip-checks=true --plugins-image=plugin-openshift-tests:v0.0.0-devel-8ff93d9 --devel-tests-image=quay.io/opct/openshift-tests:devel
	cmd.Flags().StringVar(&o.OpenshiftTestsImage, "openshift-tests-image", pkg.OpenShiftTestsImage, "Developer Mode only: openshift-tests image override")

	// Flags use for maitainance / development / CI. Those are intentionally hidden.
	cmd.Flags().StringArrayVar(o.plugins, "plugin", nil, "Override default conformance plugins to use. Can be used multiple times. (default plugins can be reviewed with assets subcommand)")
	cmd.Flags().BoolVar(&o.dedicated, "dedicated", defaultDedicatedFlag, "Setup plugins to run in dedicated test environment.")
	cmd.Flags().StringVar(&o.devCount, "dev-count", "0", "Developer Mode only: run small random set of tests. Default: 0 (disabled)")

	hideOptionalFlags(cmd, "plugin")
	hideOptionalFlags(cmd, "dedicated")
	// hideOptionalFlags(cmd, "devel-limit-tests")
	// hideOptionalFlags(cmd, "devel-skip-checks")

	hideOptionalFlags(cmd, "sonobuoy-image")
	hideOptionalFlags(cmd, "plugins-image")
	hideOptionalFlags(cmd, "collector-image")
	hideOptionalFlags(cmd, "must-gather-monitoring-image")
	hideOptionalFlags(cmd, "openshift-tests-image")

	return cmd
}

// PreRunCheck performs some checks before kicking off Sonobuoy
func (r *RunOptions) PreRunCheck(kclient kubernetes.Interface) error {
	coreClient := kclient.CoreV1()

	// Get ConfigV1 client for Cluster Operators
	restConfig, err := client.CreateRestConfig()
	if err != nil {
		return err
	}
	oc, err := coclient.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	// Check if Cluster Operators are stable
	if errs := checkClusterOperators(oc); errs != nil {
		errorMessages := []string{}
		for _, err := range errs {
			errorMessages = append(errorMessages, err.Error())
		}
		log.Errorf("Preflights checks failed: operators are not in ready state, check the status with 'oc get clusteroperator': %v", errorMessages)
		if !r.devSkipChecks {
			return errors.New("All Cluster Operators must be available, not progressing, and not degraded before validation can run.")
		}
		log.Warnf("DEVEL MODE, THIS IS NOT SUPPORTED: Skipping Cluster Operator checks: %v", errs)
	}

	// Get ConfigV1 client for Cluster Operators
	irClient, err := irclient.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	// Check if Registry is in managed state or exit
	managed, err := checkRegistry(irClient)
	if err != nil {
		if !r.devSkipChecks {
			return err
		}
		log.Warn("DEVEL MODE, THIS IS NOT SUPPORTED: Skipping Image registry check: %w", err)
	}
	if !managed {
		if !r.devSkipChecks {
			return errors.New("OpenShift Image Registry must deployed before validation can run")
		}
		log.Warn("DEVEL MODE, THIS IS NOT SUPPORTED: Skipping unmanaged image registry check")
	}

	if r.dedicated {
		log.Info("Ensuring required node label and taints exists")
		nodes, err := coreClient.Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: pkg.DedicatedNodeRoleLabelSelector,
		})
		if err != nil {
			return errors.Wrap(err, "error getting the Node list")
		}
		if len(nodes.Items) == 0 {
			return fmt.Errorf(`missing dedicated node. Set the label %q to a node and try again
Check the documentation[1] or run 'opct adm setup-node' to set the label and taints.
[1] https://redhat-openshift-ecosystem.github.io/provider-certification-tool/user/#standard-env-setup-node`, pkg.DedicatedNodeRoleLabelSelector)
		}
		if len(nodes.Items) > 2 {
			return fmt.Errorf("too many nodes with label %q. Set the label to only one node and try again", pkg.DedicatedNodeRoleLabelSelector)
		}
		node := nodes.Items[0]
		found := false
		for _, taint := range node.Spec.Taints {
			if taint.Key == pkg.DedicatedNodeRoleLabel {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing taint \"%s='':NoSchedule\" in the dedicated node %q. Set the taint and try again", pkg.DedicatedNodeRoleLabel, node.Name)
		}
	}

	// Check if namespace already exists
	p, err := coreClient.Namespaces().Get(context.TODO(), pkg.CertificationNamespace, metav1.GetOptions{})
	if err != nil {
		// If error is due to namespace not being found, we continue.
		if !kerrors.IsNotFound(err) {
			return err
		}
	}

	if p.Name != "" {
		return errors.New(fmt.Sprintf("%s namespace already exists. You must run 'destroy' to clean the environment and try again.", pkg.CertificationNamespace))
	}

	// Check if MachineConfigPool exists when upgrade mode is set.:
	// - node selectors: node-role.kubernetes.io/tests=''
	// - paused: true
	// Check MachineConfigPool when upgrade.
	if r.mode == "upgrade" {
		mcpName := "opct"
		machineConfigClient, err := mcfgclientset.NewForConfig(restConfig)
		if err != nil {
			return err
		}
		poolList, err := machineConfigClient.MachineconfigurationV1().MachineConfigPools().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("getting MachineConfigPools failed: %w", err)
		}
		// Should we need to create it when not found?
		mcpCreateInstructions := func() {
			log.Println("MachineConfigPool not found, create it with the following instructions:")
			fmt.Println(`$ cat << EOF  | oc apply -f -
---
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfigPool
metadata:
  name: opct
spec:
  machineConfigSelector:
    matchExpressions:
      - key: machineconfiguration.openshift.io/role,
        operator: In,
        values: [worker,opct]
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/tests: ""
  paused: true
EOF`)
		}
		if len(poolList.Items) == 0 {
			fmt.Println()
			return fmt.Errorf("MachineConfigPool %q not found, create it and try again", mcpName)
		}
		isFound := false
		isPaused := false
		for _, pool := range poolList.Items {
			if pool.Name == mcpName {
				isFound = true
				if !pool.Spec.Paused {
					log.Errorf("MachineConfigPool %q is not paused", mcpName)
				}
				isPaused = true
			}
		}
		if !isFound {
			mcpCreateInstructions()
			return fmt.Errorf("MachineConfigPool %q not found, create it and try again", mcpName)
		}
		if !isPaused {
			return fmt.Errorf("MachineConfigPool %q is not paused, set `spec.pause=true` and try again", mcpName)
		}
	}

	return nil
}

// PreRunSetup performs setup required by OPCT environment.
func (r *RunOptions) PreRunSetup(kclient kubernetes.Interface) error {
	rbacClient := kclient.RbacV1()

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pkg.CertificationNamespace,
			Labels:      pkg.SonobuoyDefaultLabels,
			Annotations: make(map[string]string),
		},
	}

	if r.dedicated {
		tolerations, err := json.Marshal([]v1.Toleration{{
			Key:      pkg.DedicatedNodeRoleLabel,
			Operator: v1.TolerationOpExists,
			Value:    "",
			Effect:   v1.TaintEffectNoSchedule,
		}})
		if err != nil {
			return errors.Wrap(err, "error creating namespace Tolerations")
		}

		namespace.Annotations = map[string]string{
			"openshift.io/node-selector":                       pkg.DedicatedNodeRoleLabelSelector,
			"scheduler.alpha.kubernetes.io/defaultTolerations": string(tolerations),
		}
	}

	_, err := kclient.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "error creating Namespace")
	}

	// Create Sonobuoy ServiceAccount
	// https://github.com/vmware-tanzu/sonobuoy/blob/main/pkg/client/gen.go#L611-L616
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pkg.SonobuoyServiceAccountName,
			Namespace: pkg.CertificationNamespace,
			Labels:    pkg.SonobuoyDefaultLabels,
		},
	}
	sa.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "ServiceAccount",
	})

	_, err = kclient.CoreV1().ServiceAccounts(pkg.CertificationNamespace).Create(context.TODO(), sa, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "error creating ServiceAccount")
	}

	log.Info("Ensuring the tool will run in the privileged environment...")

	// Configure custom RBAC

	// Replacing Sonobuoy's default Admin RBAC not working correctly on upgrades.
	// https://github.com/vmware-tanzu/sonobuoy/blob/5b97033257d0276c7b0d1b20412667a69d79261e/pkg/client/gen.go#L445-L481
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pkg.PrivilegedClusterRole,
			Namespace: pkg.CertificationNamespace,
			Labels:    pkg.SonobuoyDefaultLabels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				NonResourceURLs: []string{"/metrics", "/logs", "/logs/*"},
				Verbs:           []string{"get"},
			},
		},
	}
	cr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   rbacv1.GroupName,
		Version: "v1",
		Kind:    "ClusterRole",
	})

	_, err = rbacClient.ClusterRoles().Update(context.TODO(), cr, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "error creating privileged ClusterRole")
	}
	log.Infof("Created %s ClusterRole", pkg.PrivilegedClusterRole)

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pkg.PrivilegedClusterRoleBinding,
			Namespace: pkg.CertificationNamespace,
			Labels:    pkg.SonobuoyDefaultLabels,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      pkg.SonobuoyServiceAccountName,
				Namespace: pkg.CertificationNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     pkg.PrivilegedClusterRole,
		},
	}
	crb.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   rbacv1.GroupName,
		Version: "v1",
		Kind:    "ClusterRoleBinding",
	})

	_, err = rbacClient.ClusterRoleBindings().Update(context.TODO(), crb, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "error creating privileged ClusterRoleBinding")
	}
	log.Infof("Created %s ClusterRoleBinding", pkg.PrivilegedClusterRoleBinding)

	return nil
}

// createConfigMap generic way to create the configMap on the certification namespace.
func (r *RunOptions) createConfigMap(kclient kubernetes.Interface, sclient sonobuoyclient.Interface, cm *v1.ConfigMap) error {
	_, err := kclient.CoreV1().ConfigMaps(pkg.CertificationNamespace).Create(context.TODO(), cm, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Run setup and provision the certification environment.
func (r *RunOptions) Run(kclient kubernetes.Interface, sclient sonobuoyclient.Interface) error {
	var manifests []*manifest.Manifest

	imageRepository := pkg.DefaultToolsRepository
	defaultSonobuoyImage := fmt.Sprintf("%s/sonobuoy:%s", pkg.DefaultToolsRepository, buildinfo.Version)
	overrideSonobuoyImageSet := r.sonobuoyImage != defaultSonobuoyImage
	if r.imageRepository != "" {
		// sonobuoy-image override is used in dev environment to
		// test custom aggregator/worker image. Not allowed to be used in
		// production environment validated by OPCT, for that reason the instruction is to
		// mirror the sonobuoy image to /sonobuoy:version when deploying in
		// disconnected environment.
		if overrideSonobuoyImageSet {
			log.Errorf("The image override --sonobuoy-image cannot be used with --image-repository")
			os.Exit(1)
		}
		imageRepository = r.imageRepository
		log.Infof("Mirror registry is configured %s ", r.imageRepository)
	}
	if imageRepository != pkg.DefaultToolsRepository {
		log.Infof("Setting up images for custom image repository %s", imageRepository)
		r.sonobuoyImage = fmt.Sprintf("%s/%s", imageRepository, pkg.SonobuoyImage)
		r.PluginsImage = fmt.Sprintf("%s/%s", imageRepository, pkg.PluginsImage)
		r.CollectorImage = fmt.Sprintf("%s/%s", imageRepository, pkg.CollectorImage)
		r.MustGatherMonitoringImage = fmt.Sprintf("%s/%s", imageRepository, pkg.MustGatherMonitoringImage)
	}

	// Let Sonobuoy do some preflight checks before we run
	errs := sclient.PreflightChecks(&sonobuoyclient.PreflightConfig{
		Namespace:           pkg.CertificationNamespace,
		DNSNamespace:        "openshift-dns",
		DNSPodLabels:        []string{"dns.operator.openshift.io/daemonset-dns=default"},
		PreflightChecksSkip: []string{"existingnamespace"}, // Skip namespace check since we create it manually
	})
	if len(errs) > 0 {
		for _, err := range errs {
			log.Error(err)
		}
		if !r.devSkipChecks {
			return errors.New("preflight checks failed")
		}
		log.Warn("DEVEL MODE, THIS IS NOT SUPPORTED: Skipping preflight checks")
	}

	// Create version information ConfigMap
	if err := r.createConfigMap(kclient, sclient, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pkg.VersionInfoConfigMapName,
			Namespace: pkg.CertificationNamespace,
		},
		Data: map[string]string{
			"cli-version":      version.Version.Version,
			"cli-commit":       version.Version.Commit,
			"sonobuoy-version": buildinfo.Version,
			"sonobuoy-image":   r.sonobuoyImage,
		},
	}); err != nil {
		return err
	}

	configMapData := map[string]string{
		"dev-count":             r.devCount,
		"run-mode":              r.mode,
		"upgrade-target-images": r.upgradeImage,
	}

	if len(r.imageRepository) > 0 {
		configMapData["mirror-registry"] = r.imageRepository
	}

	if err := r.createConfigMap(kclient, sclient, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pkg.PluginsVarsConfigMapName,
			Namespace: pkg.CertificationNamespace,
		},
		Data: configMapData,
	}); err != nil {
		return err
	}

	if r.plugins == nil || len(*r.plugins) == 0 {
		log.Debugf("Loading default plugins")
		var err error
		manifests, err = loadPluginManifests(r)
		if err != nil {
			return err
		}
	} else {
		// User provided their own plugins at command line
		log.Debugf("Loading plugins specific at command line")
		for _, p := range *r.plugins {
			asset, err := loader.LoadDefinitionFromFile(p)
			if err != nil {
				return err
			}
			manifests = append(manifests, asset)
		}
	}

	if len(manifests) == 0 {
		return errors.New("No validation plugins to run")
	}

	// Fill out the aggregator and worker configs
	aggConfig := config.New()
	if r.timeout > 0 {
		aggConfig.Aggregation.TimeoutSeconds = r.timeout
	}
	if r.sonobuoyImage != "" {
		aggConfig.WorkerImage = r.sonobuoyImage
	}

	// Set aggregator deployment namespace
	aggConfig.Namespace = pkg.CertificationNamespace

	// Ignore Existing SA created on preflight
	aggConfig.ExistingServiceAccount = true
	aggConfig.ServiceAccountName = pkg.SonobuoyServiceAccountName
	aggConfig.SecurityContextMode = "none"

	// Fill out the Run configuration
	runConfig := &sonobuoyclient.RunConfig{
		GenConfig: sonobuoyclient.GenConfig{
			Config:             aggConfig,
			EnableRBAC:         false, // RBAC is created in preflight
			ImagePullPolicy:    config.DefaultSonobuoyPullPolicy,
			StaticPlugins:      manifests,
			PluginEnvOverrides: nil, // TODO We'll use this later
		},
	}

	err := sclient.Run(runConfig)
	return err
}

func checkClusterOperators(configClient coclient.Interface) []error {
	var result []error
	// List all Cluster Operators
	coList, err := configClient.ConfigV1().ClusterOperators().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return []error{err}
	}

	// Each Cluster Operator should be available, not progressing, and not degraded
	for _, co := range coList.Items {
		for _, cond := range co.Status.Conditions {
			switch cond.Type {
			case configv1.OperatorAvailable:
				if cond.Status == configv1.ConditionFalse {
					result = append(result, errors.Errorf("%s is unavailable", co.Name))
				}
			case configv1.OperatorProgressing:
				if cond.Status == configv1.ConditionTrue {
					result = append(result, errors.Errorf("%s is still progressing", co.Name))
				}
			case configv1.OperatorDegraded:
				if cond.Status == configv1.ConditionTrue {
					result = append(result, errors.Errorf("%s is in degraded state", co.Name))
				}
			}
		}
	}

	return result
}

// Check registry is in managed state. We assume Cluster Operator is stable.
func checkRegistry(irClient irclient.Interface) (bool, error) {
	irConfig, err := irClient.ImageregistryV1().Configs().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	if irConfig.Spec.ManagementState != operatorv1.Managed {
		return false, nil
	}

	return true, nil
}
