package baseline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/redhat-openshift-ecosystem/opct/internal/report"
	reb "github.com/redhat-openshift-ecosystem/opct/internal/report/baseline"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type baselineGetInput struct {
	platform string
	release  string
	name     string
	dump     bool
	output   string
}

var baselineGetArgs baselineGetInput
var baselineGetCmd = &cobra.Command{
	Use:     "get",
	Example: "opct adm baseline get <baseline name>",
	Short:   "Get a baseline result to be used in the review process.",
	Long: `Get a baseline result to be used in the review process.
	Baseline results are used to compare the results of the validation tests.
	Getting a baseline result is useful when you don't have access to the internet when running 'opct report' command,
	you don't need to run this command if you have access to the internet, the command will gather the correct result automatically.`,
	Run: baselineGetCmdRun,
}

func init() {
	baselineGetCmd.Flags().StringVar(&baselineGetArgs.platform, "platform", "", "Specify the platform type. Require --platform. Example: External")
	baselineGetCmd.Flags().StringVar(&baselineGetArgs.release, "release", "", "Specify the release to retrieve latest summary. Require --release. Example: 4.15")
	baselineGetCmd.Flags().StringVarP(&baselineGetArgs.name, "name", "n", "", "List result by platform. Require --platform")
	baselineGetCmd.Flags().BoolVar(&baselineGetArgs.dump, "dump", false, "Enable dump the raw data to stdout.")
	baselineGetCmd.Flags().StringVarP(&baselineGetArgs.output, "output", "o", "", "Save the baseline to output file.")
}

func baselineGetCmdRun(cmd *cobra.Command, args []string) {
	if (baselineGetArgs.platform == "" && baselineGetArgs.release == "") && baselineGetArgs.name == "" {
		if baselineGetArgs.platform == "" && baselineGetArgs.release == "" {
			log.Error("argument --platform or --release must be set when --name is not used")
			return
		}
		log.Error("argument --name <result_name> must be set. Check available baseline with 'opct adm baseline list'")
		return
	}

	var err error
	var data []byte
	rb := reb.NewBaselineReportSummary()
	if baselineGetArgs.name != "" {
		log.Infof("Getting baseline result by name: %s", baselineGetArgs.name)
		data, err = rb.GetSummaryByName(baselineGetArgs.name)
	} else {
		log.Infof("Getting latest baseline result by release and platform: %s/%s", baselineGetArgs.release, baselineGetArgs.platform)
		if err := rb.GetLatestSummaryByPlatform(baselineGetArgs.release, baselineGetArgs.platform); err != nil {
			log.Errorf("error getting latest summary by platform: %v", err)
			return
		}
		data = rb.GetBuffer().GetRawData()
	}

	if err != nil {
		log.Fatalf("Failed to read result: %v", err)
	}

	// deserialize the data to report.ReportData
	re := &report.ReportData{}
	err = json.Unmarshal(data, &re)
	if err != nil {
		log.Errorf("failed to unmarshal baseline data: %v", err)
		return
	}
	log.Infof("Baseline result processed from archive: %v", filepath.Base(re.Summary.Tests.Archive))

	if baselineGetArgs.dump {
		prettyJSON, err := json.MarshalIndent(re, "", "  ")
		if err != nil {
			log.Errorf("Failed to encode data to pretty JSON: %v", err)
		}
		if err == nil && baselineGetArgs.output != "" {
			err = os.WriteFile(baselineGetArgs.output, prettyJSON, 0644)
			if err != nil {
				log.Errorf("Failed to write pretty JSON to output file: %v", err)
			} else {
				log.Infof("Pretty JSON saved to %s\n", baselineGetArgs.output)
			}
		} else {
			fmt.Println(string(prettyJSON))
		}
	}

	// Temp getting plugin failures
	bd := reb.BaselineData{}
	bd.SetRawData(data)
	pluginName := "20-openshift-conformance-validated"
	failures, _ := bd.GetPriorityFailuresFromPlugin(pluginName)

	fmt.Println(">> Example serializing and extracting plugin failures for ", pluginName)
	for f := range failures {
		fmt.Printf("[%d]: %s\n", f, failures[f])
	}
}
