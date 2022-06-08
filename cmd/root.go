package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vmware-tanzu/sonobuoy/cmd/sonobuoy/app"

	"github.com/openshift/provider-certification-tool/pkg/assets"
	"github.com/openshift/provider-certification-tool/pkg/client"
	"github.com/openshift/provider-certification-tool/pkg/destroy"
	"github.com/openshift/provider-certification-tool/pkg/retrieve"
	"github.com/openshift/provider-certification-tool/pkg/run"
	"github.com/openshift/provider-certification-tool/pkg/status"
	"github.com/openshift/provider-certification-tool/pkg/version"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "openshift-provider-cert",
	Short: "OpenShift Provider Certification Tool",
	Long:  `OpenShift Provider Certification Tool is used to evaluate an OpenShift installation on a provider or hardware is in conformance`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		var err error

		// Validate logging level
		loglevel := viper.GetString("loglevel")
		logrusLevel, err := log.ParseLevel(loglevel)
		if err != nil {
			log.Fatal(err)
		}
		log.SetLevel(logrusLevel)

		// Additional log options
		customFormatter := new(log.TextFormatter)
		customFormatter.FullTimestamp = true
		log.SetFormatter(customFormatter)

		// Save kubeconfig
		client.Kubeconfig = viper.GetString("kubeconfig")
		if client.Kubeconfig == "" {
			log.Fatal("--kubeconfig or KUBECONFIG environment variable must be set")
		}

		// Check kubeconfig exists
		if _, err := os.Stat(client.Kubeconfig); err != nil {
			log.Fatal(err)
		}
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
	rootCmd.PersistentFlags().String("loglevel", "info", "logging level")
	viper.BindPFlag("kubeconfig", rootCmd.PersistentFlags().Lookup("kubeconfig"))
	viper.BindPFlag("loglevel", rootCmd.PersistentFlags().Lookup("loglevel"))

	// Link in child commands
	rootCmd.AddCommand(assets.NewCmdAssets())
	rootCmd.AddCommand(destroy.NewCmdDestroy())
	rootCmd.AddCommand(retrieve.NewCmdRetrieve())
	rootCmd.AddCommand(run.NewCmdRun())
	rootCmd.AddCommand(status.NewCmdStatus())
	rootCmd.AddCommand(version.NewCmdVersion())

	// Link in child commands direct from Sonobuoy
	rootCmd.AddCommand(app.NewSonobuoyCommand())
	rootCmd.AddCommand(app.NewCmdResults())
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.AutomaticEnv() // read in environment variables that match
}
