package baseline

import (
	"log"
	"os"
	"sort"

	reb "github.com/redhat-openshift-ecosystem/opct/internal/report/baseline"

	table "github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

type baselineListInput struct {
	all bool
}

var baselineListArgs baselineListInput
var baselineListCmd = &cobra.Command{
	Use:     "list",
	Example: "opct adm baseline list",
	Short:   "List all available baseline results by OpenShift version, provider and platform type.",
	Run:     baselineListCmdRun,
}

func init() {
	baselineListCmd.Flags().BoolVar(&baselineListArgs.all, "all", false, "List all results, instead of latest.")

	if baselineListArgs.all && os.Getenv("OPCT_ENABLE_ADM_BASELINE") != "1" {
		log.Fatal("You are not allowed to execute this command.")
	}
}

func baselineListCmdRun(cmd *cobra.Command, args []string) {
	rb := reb.NewBaselineReportSummary()
	index, err := rb.ReadReportSummaryIndexFromAPI()
	if err != nil {
		log.Fatalf("Failed to read index from bucket: %v", err)
	}

	tb := table.NewWriter()
	tb.SetOutputMirror(os.Stdout)
	// tbProv.SetStyle(table.StyleLight)
	// tbProv.SetTitle(title)
	if !baselineListArgs.all {
		tb.AppendHeader(table.Row{"ID", "Type", "Release", "PlatformType", "Name"})
		laltestK := make([]string, 0, len(index.Latest))
		for lts := range index.Latest {
			laltestK = append(laltestK, lts)
		}
		sort.Strings(laltestK)
		for _, latest := range laltestK {
			tb.AppendRow(
				table.Row{
					latest,
					"latest",
					index.Latest[latest].OpenShiftRelease,
					index.Latest[latest].PlatformType,
					index.Latest[latest].Name,
				})
		}
		tb.Render()
		return
	}

	tb.AppendHeader(table.Row{"Latest", "Release", "Platform", "Provider", "Name", "Version"})
	for i := range index.Results {
		res := index.Results[i]
		latest := ""
		if res.IsLatest {
			latest = "*"
		}
		provider := ""
		if p, ok := res.Tags["providerName"]; ok {
			provider = p.(string)
		}
		version := ""
		if p, ok := res.Tags["openshiftVersion"]; ok {
			version = p.(string)
		}
		tb.AppendRow(
			table.Row{
				latest,
				res.OpenShiftRelease,
				res.PlatformType,
				provider,
				res.Name,
				version,
			})
	}
	tb.Render()
}
