package pkg

import (
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	"k8s.io/client-go/kubernetes"
)

const (
	AnyUIDClusterRoleBinding     = "opct-anyuid"
	PrivilegedClusterRoleBinding = "opct-privileged"
	CertificationNamespace       = "openshift-provider-certification"
)

type Config struct {
	Kubeconfig     string
	Clientset      kubernetes.Interface
	SonobuoyClient *client.SonobuoyClient
	SonobuoyImage  string
	Timeout        int
	Watch          bool
}
