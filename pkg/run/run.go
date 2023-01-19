package run

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	coclient "github.com/openshift/client-go/config/clientset/versioned"
	irclient "github.com/openshift/client-go/imageregistry/clientset/versioned"
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
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/assets"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/client"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/status"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/pkg/wait"
)

type RunOptions struct {
	plugins       *[]string
	dedicated     bool
	sonobuoyImage string
	timeout       int
	watch         bool
	devCount      string
	mode          string
	upgradeImage  string
	devCount      string
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

func NewCmdRun() *cobra.Command {
	var err error
	var kclient kubernetes.Interface
	var sclient sonobuoyclient.Interface
	o := newRunOptions()

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the suite of tests for provider certification",
		Long:  `Launches the provider certification environment inside of an already running OpenShift cluster`,
		PreRun: func(cmd *cobra.Command, args []string) {
			// Client setup
			kclient, sclient, err = client.CreateClients()
			if err != nil {
				log.Fatal(err)
			}

			// Pre-checks and setup
			err = o.PreRunCheck(kclient)
			if err != nil {
				log.Fatal(err)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("Running OpenShift Provider Certification Tool...")

			// Fire off sonobuoy
			err := o.Run(kclient, sclient)
			if err != nil {
				log.WithError(err).Fatal("Error running the tool. Please check the errors and try again.")
			}

			log.Info("Jobs scheduled! Waiting for resources be created...")

			// Wait for Sonobuoy to create
			wait.WaitForRequiredResources(kclient)
			if err != nil {
				log.WithError(err).Fatal("error waiting for sonobuoy pods to become ready")
			}

			// Sleep to give status time to appear
			time.Sleep(status.StatusInterval)

			s := status.NewStatusOptions(o.watch)
			err = s.WaitForStatusReport(cmd.Context(), sclient)
			if err != nil {
				log.WithError(err).Fatal("error retrieving aggregator status")
			}

			err = s.Update(sclient)
			if err != nil {
				log.Fatal(err)
			}

			err = s.Print(cmd, sclient)
			if err != nil {
				log.Fatal(err)
			}

			if !o.watch {
				log.Info("Sonobuoy pods are ready!")
			}
		},
	}

	cmd.Flags().BoolVar(&o.dedicated, "dedicated", defaultDedicatedFlag, "Setup plugins to run in dedicated test environment.")
	cmd.Flags().StringVar(&o.devCount, "dev-count", "0", "Developer Mode only: run small random set of tests. Default: 0 (disabled)")
	cmd.Flags().StringVar(&o.mode, "mode", defaultRunMode, "Run mode: Availble: regular, upgrade")
	cmd.Flags().StringVar(&o.upgradeImage, "upgrade-to-image", defaultUpgradeImage, "Target OpenShift Release Image. Example: oc adm release info 4.11.18 -o jsonpath={.image}")
	cmd.Flags().StringArrayVar(o.plugins, "plugin", nil, "Override default conformance plugins to use. Can be used multiple times. (default plugins can be reviewed with assets subcommand)")
	cmd.Flags().StringVar(&o.sonobuoyImage, "sonobuoy-image", fmt.Sprintf("quay.io/ocp-cert/sonobuoy:%s", buildinfo.Version), "Image override for the Sonobuoy worker and aggregator")
	cmd.Flags().IntVar(&o.timeout, "timeout", defaultRunTimeoutSeconds, "Execution timeout in seconds")
	cmd.Flags().BoolVarP(&o.watch, "watch", "w", defaultRunWatchFlag, "Keep watch status after running")
	cmd.Flags().StringVar(&o.devCount, "dev-count", "0", "Developer Mode only: run small random set of tests. Default: 0 (disabled)")

	// Hide dedicated flag since this is for development only
	cmd.Flags().MarkHidden("dedicated")
	cmd.Flags().MarkHidden("dev-count")

	return cmd
}

// PreRunCheck performs some checks before kicking off Sonobuoy
func (r *RunOptions) PreRunCheck(kclient kubernetes.Interface) error {
	coreClient := kclient.CoreV1()
	rbacClient := kclient.RbacV1()

	// Get ConfigV1 client for Cluster Operators
	restConfig, err := client.CreateRestConfig()
	if err != nil {
		return err
	}
	configClient, err := coclient.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	// Check if Cluster Operators are stable
	errs := checkClusterOperators(configClient)
	if errs != nil {
		for _, err := range errs {
			log.Warn(err)
		}
		return errors.New("All Cluster Operators must be available, not progressing, and not degraded before certification can run")
	}

	// Get ConfigV1 client for Cluster Operators
	irClient, err := irclient.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	// Check if Registry is in managed state or exit
	managed, err := checkRegistry(irClient)
	if err != nil {
		return err
	}
	if !managed {
		return errors.New("OpenShift Image Registry must deployed before certification can run")
	}

	// // Check if MachineConfigPool exists and create
	// if err := checkCreateMCP(irClient); err != nil {
	// 	return errors.Wrap(err, "error creating MachineConfigPool")
	// }

	// Check if sonobuoy namespace already exists
	p, err := coreClient.Namespaces().Get(context.TODO(), pkg.CertificationNamespace, metav1.GetOptions{})
	if err != nil {
		// If error is due to namespace not being found, we continue.
		if !kerrors.IsNotFound(err) {
			return err
		}
	}

	// sonobuoy namespace exists so return error
	if p.Name != "" {
		return errors.New(fmt.Sprintf("%s namespace already exists", pkg.CertificationNamespace))
	}

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   pkg.CertificationNamespace,
			Labels: pkg.SonobuoyDefaultLabels,
		},
	}

	if r.dedicated {

		log.Info("Ensuring proper node label for dedicated mode exists")
		nodes, err := coreClient.Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: pkg.DedicatedNodeRoleLabelSelector,
		})
		if err != nil {
			return errors.Wrap(err, "error getting the Node list")
		}
		if nodes.Items != nil && len(nodes.Items) == 0 {
			return errors.New("No nodes with role required for dedicated mode (node-role.kubernetes.io/tests)")
		}

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
			"scheduler.alpha.kubernetes.io/defaultTolerations": string(tolerations),
			"openshift.io/node-selector":                       pkg.DedicatedNodeRoleLabelSelector,
		}
	}

	_, err = kclient.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
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

	// All good
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
		return errors.New("preflight checks failed")
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

	if err := r.createConfigMap(kclient, sclient, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pkg.PluginsVarsConfigMapName,
			Namespace: pkg.CertificationNamespace,
		},
		Data: map[string]string{
			"dev-count":             r.devCount,
			"run-mode":              r.mode,
			"upgrade-target-images": r.upgradeImage,
			"dev-count":             r.devCount,
		},
	}); err != nil {
		return err
	}

	if r.plugins == nil || len(*r.plugins) == 0 {
		// Use default built-in plugins
		log.Debugf("Loading default certification plugins")
		for _, m := range assets.AssetNames() {
			log.Debugf("Loading certification plugin: %s", m)
			asset, err := loader.LoadDefinition(assets.MustAsset(m))
			if err != nil {
				return err
			}
			manifests = append(manifests, &asset)
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
		return errors.New("No certification plugins to run")
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

// // Check or create MachineConfigPool
// func checkCreateMCP(mcpClient mcpclient.Interface) (bool, error) {
// 	irConfig, err := mcpclient.ImageregistryV1().Configs().Get(context.TODO(), "cluster", metav1.GetOptions{})
// 	if err != nil {
// 		return false, err
// 	}

// 	if irConfig.Spec.ManagementState != operatorv1.Managed {
// 		return false, nil
// 	}

// 	return true, nil
// }
