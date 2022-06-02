package client

import (
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	sonodynamic "github.com/vmware-tanzu/sonobuoy/pkg/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var Kubeconfig string

// CreateClients creates kubernetes and sonobuoy client instances
func CreateClients() (kubernetes.Interface, client.Interface, error) {
	clientConfig, err := clientcmd.BuildConfigFromFlags("", Kubeconfig)
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
