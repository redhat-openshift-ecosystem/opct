package run

import (
	"context"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	"github.com/vmware-tanzu/sonobuoy/pkg/config"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/loader"
	"github.com/vmware-tanzu/sonobuoy/pkg/plugin/manifest"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nsv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"

	"github.com/openshift/provider-certification-tool/pkg"
	"github.com/openshift/provider-certification-tool/pkg/assets"
	"github.com/openshift/provider-certification-tool/pkg/status"
	"github.com/openshift/provider-certification-tool/pkg/wait"
)

type RunOptions struct {
	config  *pkg.Config
	plugins *[]string
}

const runTimeoutSeconds = 21600

var defaultPlugins = []string{
	"manifests/openshift-kube-conformance.yaml",
	"manifests/openshift-provider-cert-level-1.yaml",
	"manifests/openshift-provider-cert-level-2.yaml",
	"manifests/openshift-provider-cert-level-3.yaml",
}

func NewRunOptions(config *pkg.Config) *RunOptions {
	return &RunOptions{
		config:  config,
		plugins: &[]string{},
	}
}

func NewCmdRun(config *pkg.Config) *cobra.Command {
	o := NewRunOptions(config)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the suite of tests for provider certification",
		Long:  `Launches the provider certification environment inside of an already running OpenShift cluster`,
		PreRun: func(cmd *cobra.Command, args []string) {
			// Pre-checks and setup
			err := o.PreRunCheck()
			if err != nil {
				log.WithError(err).Error("error running pre-checks")
				return
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("Running OpenShift Provider Certification Tool...")

			// Fire off sonobuoy
			err := o.Run()
			if err != nil {
				log.WithError(err).Error("Error running the tool. Please check the errors and try again.")
				return
			}

			log.Info("Jobs scheduled! Waiting for resources be created...")

			// Wait for Sonobuoy to create
			wait.WaitForRequiredResources(o.config)
			if err != nil {
				log.WithError(err).Error("error waiting for sonobuoy pods to become ready")
				return
			}

			s := status.NewStatusOptions(o.config)
			err = s.WaitForStatusReport(cmd.Context())
			if err != nil {
				log.WithError(err).Error("error retrieving aggregator status")
			}

			st := status.NewStatusOptions(o.config)
			st.Update()
			st.Print(cmd)

			log.Info("Sonobuoy pods are ready!")
		},
	}

	cmd.Flags().StringVar(&o.config.SonobuoyImage, "sonobuoy-image", "", "Image override for the Sonobuoy worker and aggregator")
	cmd.Flags().IntVar(&o.config.Timeout, "timeout", runTimeoutSeconds, "Execution timeout in seconds")
	cmd.Flags().BoolVarP(&o.config.Watch, "watch", "w", false, "Keep watch status after running")
	cmd.Flags().StringArrayVar(o.plugins, "plugin", nil, "Override default conformance plugins to use. Can be used multiple times (defaults to latest plugins in https://github.com/openshift/provider-certification-tool)")

	return cmd
}

// PreRunCheck performs some checks before kicking off Sonobuoy
func (r *RunOptions) PreRunCheck() error {
	nsClient, err := nsv1.NewForConfig(r.config.ClientConfig)
	if err != nil {
		return err
	}

	// Check if sonobuoy namespace already exists
	p, err := nsClient.Namespaces().Get(context.TODO(), pkg.CertificationNamespace, metav1.GetOptions{})
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

	log.Info("Ensuring the tool will run in the privileged environment...")
	// Configure SCC
	rbacClient, err := rbacv1client.NewForConfig(r.config.ClientConfig)
	if err != nil {
		return err
	}

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

func (r *RunOptions) Run() error {
	var manifests []*manifest.Manifest

	// Let Sonobuoy do some preflight checks before we run
	errs := r.config.SonobuoyClient.PreflightChecks(&client.PreflightConfig{
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

	if r.plugins == nil || len(*r.plugins) == 0 {
		// Use default built-in plugins
		log.Debugf("Loading default certification plugins")
		for _, m := range defaultPlugins {
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
	if r.config.Timeout > 0 {
		aggConfig.Aggregation.TimeoutSeconds = r.config.Timeout
	}
	if r.config.SonobuoyImage != "" {
		aggConfig.WorkerImage = r.config.SonobuoyImage
	}

	// Set aggregator deployment namespace
	aggConfig.Namespace = pkg.CertificationNamespace

	// Fill out the Run configuration
	runConfig := &client.RunConfig{
		GenConfig: client.GenConfig{
			Config:             aggConfig,
			EnableRBAC:         true, // True because OpenShift uses RBAC
			ImagePullPolicy:    config.DefaultSonobuoyPullPolicy,
			StaticPlugins:      manifests,
			PluginEnvOverrides: nil, // TODO We'll use this later
		},
	}

	err := r.config.SonobuoyClient.Run(runConfig)
	return err
}
