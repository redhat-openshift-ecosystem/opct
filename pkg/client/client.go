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

// Client is the interface to store kubernetes and sonobuoy client instances.
type Client struct {
	// KClient is the kubernetes client instance.
	KClient kubernetes.Interface
	// SClient is the sonobuoy client instance.
	SClient client.Interface
	// RestConfig is the rest config for the kubernetes client.
	RestConfig *rest.Config
}

// NewClient creates a new client instance.
func NewClient() (*Client, error) {
	clientConfig, err := createRestConfig()
	if err != nil {
		return nil, fmt.Errorf("error creating rest config: %v", err)
	}
	cli := &Client{
		RestConfig: clientConfig,
	}

	cli.KClient, err = kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return cli, fmt.Errorf("error creating kube client: %v", err)
	}

	skc, err := sonodynamic.NewAPIHelperFromRESTConfig(clientConfig)
	if err != nil {
		return cli, fmt.Errorf("error creating sonobuoy rest helper: %v", err)
	}

	cli.SClient, err = client.NewSonobuoyClient(clientConfig, skc)
	if err != nil {
		return cli, fmt.Errorf("error creating sonobuoy client: %v", err)
	}

	return cli, nil
}

func createRestConfig() (*rest.Config, error) {
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
