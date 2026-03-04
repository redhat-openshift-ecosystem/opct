// Package version contains all identifiable versioning info for
// describing the openshift provider cert project.
package version

import (
	"fmt"
	"os"

	"github.com/redhat-openshift-ecosystem/opct/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vmware-tanzu/sonobuoy/pkg/buildinfo"
)

var (
	projectName = "openshift-provider-cert"
	version     = "unknown"
	commit      = "unknown"
)

var Version = VersionContext{
	Name:    projectName,
	Version: version,
	Commit:  commit,
}

type VersionContext struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func (vc *VersionContext) String() string {
	if vc.Version == "0.0.0" {
		return fmt.Sprintf("OPCT CLI: %s+%s", vc.Version, vc.Commit)
	}
	return fmt.Sprintf("OPCT CLI: %s", vc.Version)
}

func (vc *VersionContext) stringPluginsImage() {
	fmt.Printf("Images versions:")
	fmt.Printf("\n %s (library %s)", pkg.SonobuoyImage, buildinfo.Version)
	fmt.Printf("\n %s", pkg.PluginsImage)
	fmt.Printf("\n %s", pkg.CollectorImage)
	fmt.Printf("\n %s", pkg.MustGatherMonitoringImage)
	fmt.Println("")
}

func NewCmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print opct CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(Version.String())
			Version.stringPluginsImage()
			// get cluster version if KUBECONFIG is set
			clusterVersion, err := GetClusterVersion()
			if err != nil {
				if err.Error() == "KUBECONFIG is not set" {
					os.Exit(0)
				}
				log.WithError(err).Error("failed when getting cluster version")
				os.Exit(1)
			}
			fmt.Println("")
			fmt.Printf(" OpenShift: %s\n", clusterVersion.OpenShift)
			fmt.Printf(" Kubernetes: %s\n", clusterVersion.Kubernetes)
		},
	}
}
