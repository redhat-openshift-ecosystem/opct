package cmd

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vmware-tanzu/sonobuoy/cmd/sonobuoy/app"
	"github.com/vmware-tanzu/sonobuoy/pkg/client"
	sonodynamic "github.com/vmware-tanzu/sonobuoy/pkg/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/provider-certification-tool/pkg"
	"github.com/openshift/provider-certification-tool/pkg/destroy"
	"github.com/openshift/provider-certification-tool/pkg/results"
	"github.com/openshift/provider-certification-tool/pkg/retrieve"
	"github.com/openshift/provider-certification-tool/pkg/run"
	"github.com/openshift/provider-certification-tool/pkg/status"
	"github.com/openshift/provider-certification-tool/version"
)

var (
	config = &pkg.Config{}
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "openshift-provider-cert",
	Short: "OpenShift Provider Certification Tool",
	Long:  `OpenShift Provider Certification Tool is used to evaluate an OpenShift installation on a provider or hardware is in conformance`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		config.Kubeconfig = viper.GetString("kubeconfig")
		config.ClientConfig, err = clientcmd.BuildConfigFromFlags("", config.Kubeconfig)
		if err != nil {
			return err
		}

		skc, err := sonodynamic.NewAPIHelperFromRESTConfig(config.ClientConfig)
		if err != nil {
			return errors.Wrap(err, "couldn't get sonobuoy api helper")
		}

		config.SonobuoyClient, err = client.NewSonobuoyClient(config.ClientConfig, skc)

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("kubeconfig", "", "kubeconfig for target OpenShift cluster")
	viper.BindPFlag("kubeconfig", rootCmd.PersistentFlags().Lookup("kubeconfig"))

	// Link in child commands
	rootCmd.AddCommand(destroy.NewCmdDestroy(config))
	rootCmd.AddCommand(results.NewCmdResults(config))
	rootCmd.AddCommand(retrieve.NewCmdRetrieve(config))
	rootCmd.AddCommand(run.NewCmdRun(config))
	rootCmd.AddCommand(status.NewCmdStatus(config))
	rootCmd.AddCommand(version.NewCmdVersion())

	rootCmd.AddCommand(app.NewSonobuoyCommand())
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.AutomaticEnv() // read in environment variables that match
}
