package run

import (
	"context"
	"fmt"

	projectv1 "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	sonobuoy "github.com/vmware-tanzu/sonobuoy/cmd/sonobuoy/app"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"

	"github.com/openshift/provider-certification-tool/pkg"
	"github.com/openshift/provider-certification-tool/pkg/status"
	"github.com/openshift/provider-certification-tool/pkg/wait"
)

type RunOptions struct {
	config  *pkg.Config
	plugins *[]string
}

var defaultPlugins = []string{
	"https://raw.githubusercontent.com/openshift/provider-certification-tool/mvp/tools/plugins/openshift-kube-conformance.yaml",
	"https://raw.githubusercontent.com/openshift/provider-certification-tool/mvp/tools/plugins/openshift-provider-cert-level-1.yaml",
	"https://raw.githubusercontent.com/openshift/provider-certification-tool/mvp/tools/plugins/openshift-provider-cert-level-2.yaml",
	"https://raw.githubusercontent.com/openshift/provider-certification-tool/mvp/tools/plugins/openshift-provider-cert-level-3.yaml",
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

			if len(*o.plugins) == 0 {
				o.plugins = &defaultPlugins
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
	cmd.Flags().IntVar(&o.config.Timeout, "timeout", 21600, "Execution timeout")
	cmd.Flags().BoolVarP(&o.config.Watch, "watch", "w", false, "Keep watch status after running")
	cmd.Flags().StringArrayVar(o.plugins, "plugin", nil, "Override default conformance plugins to use. Can be used multiple times (defaults to latest plugins in https://github.com/openshift/provider-certification-tool)")

	return cmd
}

// PreRunCheck performs some checks before kicking off Sonobuoy
func (r *RunOptions) PreRunCheck() error {
	projectClient, err := projectv1.NewForConfig(r.config.ClientConfig)
	if err != nil {
		return err
	}

	// Check if sonobuoy project already exists
	p, err := projectClient.Projects().Get(context.TODO(), "sonobuoy", metav1.GetOptions{})
	if err != nil {
		// If error is due to project not being found, we continue.
		if !kerrors.IsNotFound(err) {
			return err
		}
	}

	// sonobuoy project exists so return error
	if p.Name != "" {
		return errors.New("sonobuoy project already exists")
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

	// Setup Sonobuoy command call
	runCmd := sonobuoy.NewCmdRun()

	// TODO Do any of these flags need to be configurable?
	// TODO Check error result of each command below.
	// TODO Use SonobuoyClient instead?
	runCmd.Flags().Set("dns-namespace", "openshift-dns")
	runCmd.Flags().Set("dns-pod-labels", "dns.operator.openshift.io/daemonset-dns=default")
	runCmd.Flags().Set("kubeconfig", r.config.Kubeconfig)
	for _, plugin := range *r.plugins {
		runCmd.Flags().Set("plugin", plugin)
	}
	if r.config.Timeout > 0 {
		runCmd.Flags().Set("timeout", fmt.Sprint(r.config.Timeout))
	}
	if r.config.SonobuoyImage != "" {
		runCmd.Flags().Set("sonobuoy-image", r.config.SonobuoyImage)
	}

	// Clear args, otherwise they are inherited from real command line and causes failure.
	runCmd.SetArgs([]string{})

	// Execute with all the flags
	err := runCmd.Execute()
	if err != nil {
		return err
	}

	return nil
}
