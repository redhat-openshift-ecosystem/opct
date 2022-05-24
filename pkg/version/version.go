// Package version contains all identifiable versioning info for
// describing the openshift provider cert project.
package version

import (
	"fmt"

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
	return fmt.Sprintf("OpenShift Provider Certification Tool: v%s+%s", vc.Version, vc.Commit)
}
func NewCmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print provider certification tool version",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Override root cmd
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(Version.String())
			fmt.Printf("Sonobuoy Version: %s\n", buildinfo.Version)
		},
	}
}
