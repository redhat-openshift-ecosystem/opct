package client

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	sonodynamic "github.com/vmware-tanzu/sonobuoy/pkg/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func CreateRestConfig() (*rest.Config, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if len(kubeconfig) == 0 {
		kubeconfig = viper.GetString("kubeconfig")
		if kubeconfig == "" {
			return nil, fmt.Errorf("--kubeconfig or KUBECONFIG environment variable must be set")
		}

		// Check kubeconfig exists
		if _, err := os.Stat(kubeconfig); err != nil {
			return nil, fmt.Errorf("kubeconfig %q does not exists: %v", kubeconfig, err)
		}
	}

	clientConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	return clientConfig, err
}

// CreateClients creates kubernetes and sonobuoy client instances
func CreateClients() (kubernetes.Interface, client.Interface, error) {
	clientConfig, err := CreateRestConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating kube client config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating kube client: %v", err)
	}

	skc, err := sonodynamic.NewAPIHelperFromRESTConfig(clientConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating sonobuoy rest helper: %v", err)
	}

	sonobuoyClient, err := client.NewSonobuoyClient(clientConfig, skc)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating sonobuoy client: %v", err)
	}

	return clientset, sonobuoyClient, nil
}
