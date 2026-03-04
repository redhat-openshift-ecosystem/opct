package version

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/redhat-openshift-ecosystem/opct/pkg/client"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"

	coclient "github.com/openshift/client-go/config/clientset/versioned"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterVersion struct {
	OpenShift  string `json:"openshift"`
	Kubernetes string `json:"kubernetes"`
}

func GetClusterVersion() (*ClusterVersion, error) {
	fmt.Printf("Cluster version:")
	if !viper.IsSet("kubeconfig") {
		fmt.Printf(" unknown (KUBECONFIG is not set)\n")
		return nil, errors.New("KUBECONFIG is not set")
	}

	var cli *client.Client
	var err error
	cli, err = client.NewClient()
	if err != nil {
		log.WithError(err).Error("pre-run failed when creating clients")
		os.Exit(1)
	}
	oc, err := coclient.NewForConfig(cli.RestConfig)
	if err != nil {
		log.WithError(err).Error("pre-run failed when creating clients")
		os.Exit(1)
	}

	// Get OpenShift version
	cv, err := oc.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		log.Warnf("Failed to get cluster version, defaulting to kubernetes/conformance suite with openshift-tests: %v", err)
		os.Exit(1)
	}

	versionString := " unknown (unable to get cluster version)"
	version := cv.Status.Desired.Version
	if version != "" {
		versionString = version
	}

	// Retrieve kubernetes version
	kubeClient, err := kubernetes.NewForConfig(cli.RestConfig)
	if err != nil {
		log.WithError(err).Error("pre-run failed when creating clients")
		os.Exit(1)
	}
	kubeVersion, err := kubeClient.Discovery().ServerVersion()
	if err != nil {
		log.WithError(err).Error("pre-run failed when creating clients")
		os.Exit(1)
	}

	return &ClusterVersion{
		OpenShift:  versionString,
		Kubernetes: kubeVersion.String(),
	}, nil
}
