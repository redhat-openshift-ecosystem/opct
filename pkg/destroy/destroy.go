package destroy

import (
	"context"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	sonobuoyclient "github.com/vmware-tanzu/sonobuoy/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/redhat-openshift-ecosystem/opct/pkg"
	"github.com/redhat-openshift-ecosystem/opct/pkg/client"
)

const (
	DeleteSonobuoyEnvWaitTime = time.Hour * 1
	NonOpenShiftNamespace     = "e2e-.*"
)

type DestroyOptions struct {
}

func NewDestroyOptions() *DestroyOptions {
	return &DestroyOptions{}
}

func NewCmdDestroy() *cobra.Command {
	o := NewDestroyOptions()
	cmd := &cobra.Command{
		Use:     "destroy",
		Aliases: []string{"delete"},
		Short:   "Destroy current validation environment",
		Long:    `Destroys resources used for conformance testing inside of the target OpenShift cluster. This will not destroy OpenShift cluster.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("Starting the destroy flow...")

			// Client setup
			kclient, sclient, err := client.CreateClients()
			if err != nil {
				log.Error(err)
				return err
			}

			log.Info("removing opct namespace...")
			if err := o.DeleteSonobuoyEnv(sclient); err != nil {
				log.Warn(err)
			}

			log.Info("removing stale e2e namespaces...")
			if err := o.DeleteTestNamespaces(kclient); err != nil {
				log.Warn(err)
			}

			log.Info("restoring privileged environment...")
			if err := o.RestoreSCC(kclient); err != nil {
				log.Warn(err)
			}

			log.Info("Destroy done!")
			return nil
		},
	}

	return cmd
}

// DeleteSonobuoyEnv initiates deletion of Sonobuoy environment and waits until completion.
func (d *DestroyOptions) DeleteSonobuoyEnv(sclient sonobuoyclient.Interface) error {
	deleteConfig := &sonobuoyclient.DeleteConfig{
		Namespace: pkg.CertificationNamespace,
		Wait:      DeleteSonobuoyEnvWaitTime,
	}

	return sclient.Delete(deleteConfig)
}

// DeleteTestNamespaces deletes any non-openshift namespace.
func (d *DestroyOptions) DeleteTestNamespaces(kclient kubernetes.Interface) error {
	client := kclient.CoreV1()

	// Get list of all namespaces (TODO is there way to filter these server-side?)
	nsList, err := client.Namespaces().List(context.TODO(), metav1.ListOptions{})
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
		err := client.Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})
		if err != nil {
			log.WithError(err).Warnf("error deleting namespace %s", ns)
		}
	}

	return nil
}

func (d *DestroyOptions) RestoreSCC(kclient kubernetes.Interface) error {
	client := kclient.RbacV1()

	err := client.ClusterRoles().Delete(context.TODO(), pkg.PrivilegedClusterRole, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	log.Infof("Deleted %s ClusterRole", pkg.PrivilegedClusterRole)

	err = client.ClusterRoleBindings().Delete(context.TODO(), pkg.PrivilegedClusterRoleBinding, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	log.Infof("Deleted %s ClusterRoleBinding", pkg.PrivilegedClusterRoleBinding)

	return nil
}
