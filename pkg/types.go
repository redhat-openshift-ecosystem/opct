package pkg

import (
	"os"

	"github.com/adrg/xdg"
	log "github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	"k8s.io/client-go/rest"
)

const (
	AnyUIDClusterRoleBinding     = "opct-anyuid"
	PrivilegedClusterRoleBinding = "opct-privileged"
	ResultsFileName              = "results-latest.txt"
)

var (
	ResultsDirectory string
)

func init() {
	var err error
	ResultsDirectory, err = xdg.CacheFile("opct")
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

type Config struct {
	Kubeconfig     string
	ClientConfig   *rest.Config
	SonobuoyClient *client.SonobuoyClient
	SonobuoyImage  string
	Timeout        int
	Watch          bool
}
