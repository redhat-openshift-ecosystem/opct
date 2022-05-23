package destroy

import (
	"context"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nsv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"

	"github.com/openshift/provider-certification-tool/pkg"
)

const (
	DeleteSonobuoyEnvWaitTime = time.Hour * 1
	NonOpenShiftNamespace     = "e2e-.*"
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
		Namespace: pkg.CertificationNamespace,
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
	nsClient, err := nsv1.NewForConfig(d.config.ClientConfig)
	if err != nil {
		return err
	}

	// Get list of all namespaces (TODO is there way to filter these server-side?)
	nsList, err := nsClient.Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Filter namespaces by name
	var nonOpenShiftNamespaces []string
	pattern := regexp.MustCompile(NonOpenShiftNamespace)
	for _, ns := range nsList.Items {
		if pattern.MatchString(ns.Name) {
			log.Infof("stale namespace was found: %s, removing...", ns.Name)
			nonOpenShiftNamespaces = append(nonOpenShiftNamespaces, ns.Name)
		}
	}

	// Delete filtered namespaces
	for _, ns := range nonOpenShiftNamespaces {
		err := nsClient.Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})
		if err != nil {
			log.WithError(err).Warnf("error deleting namespace %s", ns)
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
