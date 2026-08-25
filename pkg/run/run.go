package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	coclient "github.com/openshift/client-go/config/clientset/versioned"
	irclient "github.com/openshift/client-go/imageregistry/clientset/versioned"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/redhat-openshift-ecosystem/opct/pkg/version"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vmware-tanzu/sonobuoy/pkg/buildinfo"
	sonobuoyclient "github.com/vmware-tanzu/sonobuoy/pkg/client"
	"github.com/vmware-tanzu/sonobuoy/pkg/config"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/loader"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/manifest"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/redhat-openshift-ecosystem/opct/pkg"
	"github.com/redhat-openshift-ecosystem/opct/pkg/client"
	"github.com/redhat-openshift-ecosystem/opct/pkg/status"
	"github.com/redhat-openshift-ecosystem/opct/pkg/wait"
	rbacv1 "k8s.io/api/rbac/v1"
	kresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

type RunOptions struct {
	plugins *[]string

	sonobuoyImage   string
	imageRepository string

	// PluginsImage
	// defines the image containing plugins associated with the opct.
	// this variable is referenced by plugin manifest templates to dynamically reference the plugins image.
	PluginsImage              string
	CollectorImage            string
	MustGatherMonitoringImage string
	OpenshiftTestsImage       string

	timeout      int
	watch        bool
	mode         string
	upgradeImage string

	// Pre-run validation retry/timeout configuration
	validationTimeout       int
	validationRetryInterval int

	// devel flags
	devCount      string
	devSkipChecks bool

	// Dedicated node
	dedicated bool

	// dryRun flag - when true, only run preflight checks without creating resources
	dryRun bool

	// verbose flag - when true, print rendered plugin manifests to stdout
	verbose bool
}

const (
	defaultRunTimeoutSeconds              = 21600
	defaultRunMode                        = "regular"
	defaultUpgradeImage                   = ""
	defaultDedicatedFlag                  = true
	defaultRunWatchFlag                   = false
	defaultDryRunFlag                     = false
	defaultVerboseFlag                    = false
	defaultValidationTimeoutSeconds       = 600 // 10 minutes for all pre-run validations
	defaultValidationRetryIntervalSeconds = 10  // 10 seconds between validation retries
)

// IsUpgradeMode returns true when the run mode is upgrade. Exported for use
// in plugin manifest Go templates where unexported fields are inaccessible.
func (r *RunOptions) IsUpgradeMode() bool {
	return r.mode == plugin.WorkflowUpgrade
}

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
	o := newRunOptions()
	var cli *client.Client

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the suite of tests for provider validation",
		Long:  `Launches the provider validation environment inside of an already running OpenShift cluster`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Client setup
			var err error
			cli, err = client.NewClient()
			if err != nil {
				log.WithError(err).Error("pre-run failed when creating clients")
				return err
			}

			// Pre-run validations
			if errs := o.PreRunValidations(cli.KClient, cli.RestConfig); len(errs) > 0 {
				return fmt.Errorf("pre-run validation failed with %d errors, fix it and try again", len(errs))
			}

			// Pre-run setup
			if err = o.PreRunSetup(cli.KClient); err != nil {
				log.WithError(err).Error("pre-run failed when initializing the environment")
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("Running OPCT...")
			if err := o.Run(cli); err != nil {
				log.WithError(err).Errorf("execution finished with errors.")
				return err
			}

			if o.dryRun {
				log.Info("Exiting without creating resources (use 'opct run' without --dry-run to execute tests)")
				return nil
			}

			log.Info("Jobs scheduled! Waiting for resources be created...")
			if err := wait.WaitForRequiredResources(cli.KClient); err != nil {
				log.WithError(err).Errorf("error waiting for required pods to become ready")
				return err
			}

			// Retrieve the first status and print it, finishing when --watch is not set.
			s := status.NewStatus(&status.StatusInput{
				Watch:   o.watch,
				KClient: cli.KClient,
				SClient: cli.SClient,
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
	cmd.Flags().StringVar(&o.imageRepository, "image-repository", "", "Image repository containing required images test environment. Example: --mirror-repository mirror.repository.net/opct")

	cmd.Flags().IntVar(&o.timeout, "timeout", defaultRunTimeoutSeconds, "Execution timeout in seconds")
	cmd.Flags().BoolVarP(&o.watch, "watch", "w", defaultRunWatchFlag, "Keep watch status after running")

	// Pre-run validation retry/timeout configuration (hidden, for advanced use)
	cmd.Flags().IntVar(&o.validationTimeout, "validation-timeout", defaultValidationTimeoutSeconds, "Timeout for pre-run validation checks in seconds (applies to all validation checks)")
	cmd.Flags().IntVar(&o.validationRetryInterval, "validation-retry-interval", defaultValidationRetryIntervalSeconds, "Retry interval for pre-run validation checks in seconds")

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
	cmd.Flags().BoolVar(&o.dryRun, "dry-run", defaultDryRunFlag, "Run preflight checks only without creating resources")
	cmd.Flags().BoolVarP(&o.verbose, "verbose", "v", defaultVerboseFlag, "Print rendered plugin manifests to stdout")
	cmd.Flags().BoolVar(&o.devSkipChecks, "devel-skip-checks", false, "Developer Mode only: skip checks")

	// dev-count is an alias of devel-limit-tests (both flags bind to the same variable).
	// TODO(mtulio): remove this flag in the future.
	cmd.Flags().StringVar(&o.devCount, "devel-limit-tests", "0", "Developer Mode only: run small random set of tests. Default: 0 (disabled)")
	cmd.Flags().StringVar(&o.devCount, "dev-count", "0", "Developer Mode only: run small random set of tests. Default: 0 (disabled)")

	hideOptionalFlags(cmd, "plugin")
	hideOptionalFlags(cmd, "dedicated")
	hideOptionalFlags(cmd, "validation-timeout")
	hideOptionalFlags(cmd, "validation-retry-interval")
	// hideOptionalFlags(cmd, "devel-limit-tests")
	// hideOptionalFlags(cmd, "devel-skip-checks")

	hideOptionalFlags(cmd, "sonobuoy-image")
	hideOptionalFlags(cmd, "plugins-image")
	hideOptionalFlags(cmd, "collector-image")
	hideOptionalFlags(cmd, "must-gather-monitoring-image")
	hideOptionalFlags(cmd, "openshift-tests-image")

	return cmd
}

// PreRunSetup performs setup required by OPCT environment.
func (r *RunOptions) PreRunSetup(kclient kubernetes.Interface) error {

	if r.dryRun {
		log.Warnf("Dry-run mode enabled: skipping setup, resources will not be created")
		return nil
	}

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
			return fmt.Errorf("error creating namespace Tolerations: %w", err)
		}

		namespace.Annotations = map[string]string{
			"openshift.io/node-selector":                       pkg.DedicatedNodeRoleLabelSelector,
			"scheduler.alpha.kubernetes.io/defaultTolerations": string(tolerations),
		}
	}

	_, err := kclient.CoreV1().Namespaces().Create(context.TODO(), namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating Namespace: %w", err)
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
		return fmt.Errorf("error creating ServiceAccount: %w", err)
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
		return fmt.Errorf("error creating privileged ClusterRole: %w", err)
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
		return fmt.Errorf("error creating privileged ClusterRoleBinding: %w", err)
	}
	log.Infof("Created %s ClusterRoleBinding", pkg.PrivilegedClusterRoleBinding)

	// Create the Deployment of dedicated-e2e-controller.
	// The controller watches for new pods failed to schedule, and apply
	// tolerations.
	// TODO(mtulio): change the image registry to dynamic to support disconnected setup.
	if r.dedicated {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pkg.DedicatedControllerName,
				Namespace: pkg.CertificationNamespace,
				Labels:    pkg.SonobuoyDefaultLabels,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To[int32](1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": pkg.DedicatedControllerName,
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": pkg.DedicatedControllerName,
						},
					},
					Spec: v1.PodSpec{
						ServiceAccountName: pkg.SonobuoyServiceAccountName,
						Containers: []v1.Container{
							{
								Name:            "controller",
								Image:           pkg.ControllerImage,
								ImagePullPolicy: v1.PullAlways,
								Command: []string{
									"opct",
									"adm",
									"e2e-dedicated",
									"controller",
								},
								Resources: v1.ResourceRequirements{
									Limits: v1.ResourceList{
										v1.ResourceCPU:    kresource.MustParse("128m"),
										v1.ResourceMemory: kresource.MustParse("256Mi"),
									},
									Requests: v1.ResourceList{
										v1.ResourceMemory: kresource.MustParse("64Mi"),
									},
								},
							},
						},
					},
				},
			},
		}

		_, err := kclient.AppsV1().Deployments(pkg.CertificationNamespace).Create(context.TODO(), deployment, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating e2e-dedicated-controller deployment: %w", err)
		}
	}

	return nil
}

// createConfigMap generic way to create the configMap on the certification namespace.
func (r *RunOptions) createConfigMap(kclient kubernetes.Interface, cm *v1.ConfigMap) error {
	_, err := kclient.CoreV1().ConfigMaps(pkg.CertificationNamespace).Create(context.TODO(), cm, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Run setup and provision the certification environment.
func (r *RunOptions) Run(cli *client.Client) error {
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
	errs := cli.SClient.PreflightChecks(&sonobuoyclient.PreflightConfig{
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
			return fmt.Errorf("preflight checks failed")
		}
		log.Warn("DEVEL MODE, THIS IS NOT SUPPORTED: Skipping preflight checks")
	}

	// Create environment configuration ConfigMaps
	configVersion := &v1.ConfigMap{
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
	}
	configPlugins := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pkg.PluginsVarsConfigMapName,
			Namespace: pkg.CertificationNamespace,
		},
		Data: map[string]string{
			"dev-count":             r.devCount,
			"run-mode":              r.mode,
			"upgrade-target-images": r.upgradeImage,
		},
	}
	if len(r.imageRepository) > 0 {
		configPlugins.Data["mirror-registry"] = r.imageRepository
	}
	if err := r.setSuiteName(cli, configPlugins.Data); err != nil {
		return err
	}

	if r.verbose {
		versionJSON, err := json.MarshalIndent(configVersion, "", "  ")
		if err != nil {
			log.Warnf("Unable to marshal ConfigMap %s for printing: %v", configVersion.Name, err)
		} else {
			fmt.Printf("\n---\n# ConfigMap: %s\n---\n%s\n", configVersion.Name, string(versionJSON))
		}

		pluginsJSON, err := json.MarshalIndent(configPlugins, "", "  ")
		if err != nil {
			log.Warnf("Unable to marshal ConfigMap %s for printing: %v", configPlugins.Name, err)
		} else {
			fmt.Printf("\n---\n# ConfigMap: %s\n---\n%s\n", configPlugins.Name, string(pluginsJSON))
		}
	}

	if r.dryRun {
		log.Debugf("Dry-run mode enabled, skipping creation of environment configuration ConfigMaps")
	} else {
		if err := r.createConfigMap(cli.KClient, configVersion); err != nil {
			return err
		}
		if err := r.createConfigMap(cli.KClient, configPlugins); err != nil {
			return err
		}
	}

	// Loading Plugins configurations / manifests
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
			// Print custom plugin manifest if flag is enabled
			if r.verbose {
				pluginData, err := os.ReadFile(p)
				if err != nil {
					log.Warnf("Unable to read plugin file for printing: %s: %v", p, err)
				} else {
					fmt.Printf("\n---\n# Custom plugin manifest: %s\n---\n%s\n", p, string(pluginData))
				}
			}

			asset, err := loader.LoadDefinitionFromFile(p)
			if err != nil {
				return err
			}
			manifests = append(manifests, asset)
		}
	}

	if len(manifests) == 0 {
		return fmt.Errorf("no validation plugins to run")
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

	// If dry-run mode is enabled, exit before running tests
	if r.dryRun {
		log.Debugf("Dry-run mode enabled: exiting before running tests")
		return nil
	}

	err := cli.SClient.Run(runConfig)
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
					result = append(result, fmt.Errorf("%s is unavailable", co.Name))
				}
			case configv1.OperatorProgressing:
				if cond.Status == configv1.ConditionTrue {
					result = append(result, fmt.Errorf("%s is still progressing", co.Name))
				}
			case configv1.OperatorDegraded:
				if cond.Status == configv1.ConditionTrue {
					result = append(result, fmt.Errorf("%s is in degraded state", co.Name))
				}
			}
		}
	}

	return result
}

// waitForClusterOperators waits for all cluster operators to be ready with retry logic.
// It polls cluster operators at the specified interval until all are ready or timeout is reached.
func waitForClusterOperators(ctx context.Context, configClient coclient.Interface, retryInterval time.Duration) error {
	log.Info("Waiting for cluster operators to become ready...")

	var lastErrors []error
	attempt := 0

	err := utilwait.PollUntilContextCancel(ctx, retryInterval, true, func(ctx context.Context) (bool, error) {
		attempt++

		// Check cluster operators
		errs := checkClusterOperators(configClient)

		if len(errs) == 0 {
			// All operators are ready
			log.Info("All cluster operators are ready")
			return true, nil
		}

		// Store errors for final reporting
		lastErrors = errs

		// Log transient failures as warnings
		if attempt == 1 {
			log.Warnf("Cluster operators are not ready yet (attempt %d), will retry every %s", attempt, retryInterval)
		} else {
			log.Debugf("Cluster operators still not ready (attempt %d): %d operator(s) not in ready state", attempt, len(errs))
		}

		// Log details of which operators are not ready (only at debug level to avoid spam)
		for _, err := range errs {
			log.Debugf("  - %v", err)
		}

		// Continue polling
		return false, nil
	})

	if err != nil {
		// Check if it's a timeout
		if err == context.DeadlineExceeded {
			// Provide detailed error message with last known state
			log.Errorf("Timeout waiting for cluster operators to become ready after %d attempts", attempt)
			log.Error("The following operators are not in ready state:")
			for _, opErr := range lastErrors {
				log.Errorf("  - %v", opErr)
			}
			return fmt.Errorf("timeout waiting for cluster operators to become ready: %d operator(s) still not ready after %d attempts: %v", len(lastErrors), attempt, lastErrors)
		}
		return err
	}

	return nil
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

// setSuiteName sets the suiteNameKubernetesConformance in the configMapData based on the cluster version.
func (r *RunOptions) setSuiteName(cli *client.Client, configMapData map[string]string) error {
	// Gather cluster version to check if it is 4.20+
	// If 4.20+, set the suiteNameKubernetesConformance in the configMapData to "kubernetes/conformance/parallel",
	// otherwise set it to "kubernetes/conformance".
	// https://issues.redhat.com/browse/OCPBUGS-66219

	// Get the cluster version
	oc, err := coclient.NewForConfig(cli.RestConfig)
	if err != nil {
		return fmt.Errorf("error creating config client: %w", err)
	}
	cv, err := oc.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		log.Warnf("Failed to get cluster version, defaulting to kubernetes/conformance suite with openshift-tests: %v", err)
		return nil
	}

	// Extract the version string
	version := cv.Status.Desired.Version
	if version == "" {
		log.Warn("Cluster version is empty, defaulting to kubernetes/conformance suite with openshift-tests")
		return nil
	}

	log.Debugf("Detected cluster version: %s", version)

	// Parse the version to check if it's >= 4.20
	// Version format is typically "4.20.0" or "4.20.0-rc.1"
	var major, minor int
	_, err = fmt.Sscanf(version, "%d.%d", &major, &minor)
	if err != nil {
		log.Warnf("Failed to parse cluster version %q, defaulting to kubernetes/conformance suite with openshift-tests: %v", version, err)
		return nil
	}

	// For OCP 4.20+, use k8s-tests-ext binary with parallel sub-suite
	configMapData["suiteNameKubernetesConformance"] = resolveKubernetesSuiteName(major, minor)
	log.Infof("Setting configMapData[suiteNameKubernetesConformance] to %s for OCP %d.%d", configMapData["suiteNameKubernetesConformance"], major, minor)

	return nil
}

// resolveKubernetesSuiteName returns the Kubernetes conformance suite name
// that includes the Conformance label filter for the given OCP version.
// 4.20 uses kubernetes/conformance/parallel/minimal because the umbrella suite
// (k8s#2708) has not been backported there yet. All other versions use
// kubernetes/conformance: built into openshift-tests on ≤4.19, and available
// via k8s-tests-ext (OTE) on 4.21+.
func resolveKubernetesSuiteName(major, minor int) string {
	if major == 4 && minor == 20 {
		return "kubernetes/conformance/parallel/minimal"
	}
	return "kubernetes/conformance"
}
