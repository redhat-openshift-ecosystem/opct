package destroy

import (
	"context"
	"regexp"
	"time"

	projectv1 "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"

	"github.com/openshift/provider-certification-tool/pkg"
)

const (
	DeleteSonobuoyEnvWaitTime = time.Hour * 1
	NonOpenShiftProject       = "(openshift)|(kube-(system|public|node-lease))|(default)"
)

type DestroyOptions struct {
	config *pkg.Config
}

func NewDestroyOptions(config *pkg.Config) *DestroyOptions {
	return &DestroyOptions{
		config: config,
	}
}

func NewCmdDestroy(config *pkg.Config) *cobra.Command {
	o := NewDestroyOptions(config)

	cmd := &cobra.Command{
		Use:     "destroy",
		Aliases: []string{"delete"},
		Short:   "Destroy current Certification Environment",
		Long:    `Destroys resources used for conformance testing inside of the target OpenShift cluster. This will not destroy OpenShift cluster.`,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("Starting the destroy flow...")

			// TODO Should we exit on error anywhere below?

			err := o.DeleteSonobuoyEnv()
			if err != nil {
				log.Warn(err)
			}

			err = o.DeleteStateFile()
			if err != nil {
				log.Warn(err)
			}

			log.Info("removing non-openshift NS...")
			err = o.DeleteTestNamespaces()
			if err != nil {
				log.Warn(err)
			}

			log.Info("restoring privileged environment...")
			err = o.RestoreSCC()
			if err != nil {
				log.Warn(err)
			}

			log.Info("Destroy done!")
		},
	}

	return cmd
}

// DeleteSonobuoyEnv initiates deletion of Sonobuoy environment and waits until completion.
func (d *DestroyOptions) DeleteSonobuoyEnv() error {
	deleteConfig := &client.DeleteConfig{
		Namespace: "sonobuoy",
		Wait:      DeleteSonobuoyEnvWaitTime,
	}

	return d.config.SonobuoyClient.Delete(deleteConfig)
}

// DeleteStateFile deletes the on-disk saved state.
// TODO Implement on-disk saved state
func (d *DestroyOptions) DeleteStateFile() error {
	log.Warn("DeleteStateFile not implemented yet")
	return nil
}

// DeleteTestNamespaces deletes any non-openshift namespace.
func (d *DestroyOptions) DeleteTestNamespaces() error {
	projectClient, err := projectv1.NewForConfig(d.config.ClientConfig)
	if err != nil {
		return err
	}

	// Get list of all projects (TODO is there way to filter these server-side?)
	projectList, err := projectClient.Projects().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Filter projects by name
	var nonOpenShiftProjects []string
	pattern := regexp.MustCompile(NonOpenShiftProject)
	for _, project := range projectList.Items {
		if !pattern.MatchString(project.Name) {
			log.Infof("stale namespace was found: %s, removing...", project.Name)
			nonOpenShiftProjects = append(nonOpenShiftProjects, project.Name)
		}
	}

	// Delete filtered projects
	for _, project := range nonOpenShiftProjects {
		err := projectClient.Projects().Delete(context.TODO(), project, metav1.DeleteOptions{})
		if err != nil {
			log.WithError(err).Warnf("error deleting namespace %s", project)
		}
	}

	return nil
}

func (d *DestroyOptions) RestoreSCC() error {
	rbacClient, err := rbacv1client.NewForConfig(d.config.ClientConfig)
	if err != nil {
		return err
	}

	err = rbacClient.ClusterRoleBindings().Delete(context.TODO(), pkg.AnyUIDClusterRoleBinding, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	log.Infof("Deleted %s ClusterRoleBinding", pkg.AnyUIDClusterRoleBinding)

	err = rbacClient.ClusterRoleBindings().Delete(context.TODO(), pkg.PrivilegedClusterRoleBinding, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	log.Infof("Deleted %s ClusterRoleBinding", pkg.PrivilegedClusterRoleBinding)

	return nil
}
