// Package version contains all identifiable versioning info for
// describing the openshift provider cert project.
package version

import (
	"fmt"

	"github.com/redhat-openshift-ecosystem/opct/pkg"
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
	return fmt.Sprintf("OPCT CLI: %s+%s", vc.Version, vc.Commit)
}

func (vc *VersionContext) StringPlugins() string {
	return fmt.Sprintf("OPCT Plugins: %s", pkg.PluginsImage)
}

func NewCmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print provider validation tool version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(Version.String())
			fmt.Println(Version.StringPlugins())
			fmt.Printf("Sonobuoy: %s\n", buildinfo.Version)
		},
	}
}
