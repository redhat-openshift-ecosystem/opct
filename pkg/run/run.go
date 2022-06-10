package run

import (
	"context"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	coclient "github.com/openshift/client-go/config/clientset/versioned"
	irclient "github.com/openshift/client-go/imageregistry/clientset/versioned"
	"github.com/pkg/errors"
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
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/provider-certification-tool/pkg"
	"github.com/openshift/provider-certification-tool/pkg/assets"
	"github.com/openshift/provider-certification-tool/pkg/client"
	"github.com/openshift/provider-certification-tool/pkg/status"
	"github.com/openshift/provider-certification-tool/pkg/wait"
)

type RunOptions struct {
	plugins       *[]string
	dedicated     bool
	sonobuoyImage string
	timeout       int
	watch         bool
}

const runTimeoutSeconds = 21600

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
				log.WithError(err).Error("Error running the tool. Please check the errors and try again.")
				return
			}

			log.Info("Jobs scheduled! Waiting for resources be created...")

			// Wait for Sonobuoy to create
			wait.WaitForRequiredResources(kclient)
			if err != nil {
				log.WithError(err).Error("error waiting for sonobuoy pods to become ready")
				return
			}

			s := status.NewStatusOptions(o.watch)
			err = s.WaitForStatusReport(cmd.Context(), sclient)
			if err != nil {
				log.WithError(err).Error("error retrieving aggregator status")
			}

			// TODO Why's there a second StatusOptions instance?
			st := status.NewStatusOptions(o.watch)
			st.Update(sclient)
			st.Print(cmd, sclient)

			log.Info("Sonobuoy pods are ready!")
		},
	}

	cmd.Flags().BoolVar(&o.dedicated, "dedicated", false, "Setup plugins to run in dedicated test environment.")
	cmd.Flags().StringArrayVar(o.plugins, "plugin", nil, "Override default conformance plugins to use. Can be used multiple times (defaults to latest plugins in https://github.com/openshift/provider-certification-tool)")
	cmd.Flags().StringVar(&o.sonobuoyImage, "sonobuoy-image", fmt.Sprintf("quay.io/ocp-cert/sonobuoy:%s", buildinfo.Version), "Image override for the Sonobuoy worker and aggregator")
	cmd.Flags().IntVar(&o.timeout, "timeout", runTimeoutSeconds, "Execution timeout in seconds")
	cmd.Flags().BoolVarP(&o.watch, "watch", "w", false, "Keep watch status after running")

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
		return errors.New("sonobuoy namespace already exists")
	}

	if r.dedicated {
		log.Info("Ensuring proper node label for dedicated mode")
		nodes, err := coreClient.Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/tests=",
		})
		if err != nil {
			return err
		}
		if nodes.Items != nil && len(nodes.Items) == 0 {
			return errors.New("No nodes with role required for dedicated mode (node-role.kubernetes.io/tests)")
		}
	}

	log.Info("Ensuring the tool will run in the privileged environment...")
	// Configure SCC
	anyuid := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: pkg.AnyUIDClusterRoleBinding,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     rbacv1.GroupKind,
				APIGroup: rbacv1.GroupName,
				Name:     "system:authenticated",
			},
			{
				Kind:     rbacv1.GroupKind,
				APIGroup: rbacv1.GroupName,
				Name:     "system:serviceaccounts",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:openshift:scc:anyuid",
		},
	}

	privileged := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: pkg.PrivilegedClusterRoleBinding,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     rbacv1.GroupKind,
				APIGroup: rbacv1.GroupName,
				Name:     "system:authenticated",
			},
			{
				Kind:     rbacv1.GroupKind,
				APIGroup: rbacv1.GroupName,
				Name:     "system:serviceaccounts",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:openshift:scc:privileged",
		},
	}

	_, err = rbacClient.ClusterRoleBindings().Update(context.TODO(), anyuid, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "error creating anyuid ClusterRoleBinding")
	}
	log.Infof("Created %s ClusterRoleBinding", pkg.AnyUIDClusterRoleBinding)

	_, err = rbacClient.ClusterRoleBindings().Update(context.TODO(), privileged, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "error creating privileged ClusterRoleBinding")
	}
	log.Infof("Created %s ClusterRoleBinding", pkg.PrivilegedClusterRoleBinding)

	// All good
	return nil
}

func (r *RunOptions) Run(kclient kubernetes.Interface, sclient sonobuoyclient.Interface) error {
	var manifests []*manifest.Manifest

	if r.dedicated {
		// Skip preflight checks and create namespace manually with Tolerations
		tolerations, err := json.Marshal([]v1.Toleration{{
			Key:      "node-role.kubernetes.io/tests",
			Operator: v1.TolerationOpExists,
			Value:    "",
			Effect:   v1.TaintEffectNoSchedule,
		}})
		if err != nil {
			return err
		}

		dedicatedNamespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: pkg.CertificationNamespace,
				Annotations: map[string]string{
					"scheduler.alpha.kubernetes.io/defaultTolerations": string(tolerations),
					"openshift.io/node-selector":                       "node-role.kubernetes.io/tests=",
				},
			},
		}

		_, err = kclient.CoreV1().Namespaces().Create(context.TODO(), dedicatedNamespace, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		// Let Sonobuoy do some preflight checks before we run
		errs := sclient.PreflightChecks(&sonobuoyclient.PreflightConfig{
			Namespace:    pkg.CertificationNamespace,
			DNSNamespace: "openshift-dns",
			DNSPodLabels: []string{"dns.operator.openshift.io/daemonset-dns=default"},
		})
		if len(errs) > 0 {
			for _, err := range errs {
				log.Error(err)
			}
			return errors.New("preflight checks failed")
		}
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

	// Fill out the Run configuration
	runConfig := &sonobuoyclient.RunConfig{
		GenConfig: sonobuoyclient.GenConfig{
			Config:             aggConfig,
			EnableRBAC:         true, // True because OpenShift uses RBAC
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
