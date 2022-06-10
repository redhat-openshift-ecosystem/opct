package client

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	sonodynamic "github.com/vmware-tanzu/sonobuoy/pkg/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var Kubeconfig string

func CreateRestConfig() (*rest.Config, error) {

	// Singleton kubeconfig
	if Kubeconfig == "" {
		Kubeconfig = viper.GetString("kubeconfig")
		if Kubeconfig == "" {
			log.Fatal("--kubeconfig or KUBECONFIG environment variable must be set")
		}

		// Check kubeconfig exists
		if _, err := os.Stat(Kubeconfig); err != nil {
			log.Fatal(err)
		}
	}

	clientConfig, err := clientcmd.BuildConfigFromFlags("", Kubeconfig)
	return clientConfig, err
}

// CreateClients creates kubernetes and sonobuoy client instances
func CreateClients() (kubernetes.Interface, client.Interface, error) {
	clientConfig, err := CreateRestConfig()
	if err != nil {
		return nil, nil, err
	}

	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, err
	}

	skc, err := sonodynamic.NewAPIHelperFromRESTConfig(clientConfig)
	if err != nil {
		return nil, nil, err
	}

	sonobuoyClient, err := client.NewSonobuoyClient(clientConfig, skc)
	if err != nil {
		return nil, nil, err
	}

	return clientset, sonobuoyClient, nil
}
