package report

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"text/tabwriter"

	table "github.com/jedib0t/go-pretty/v6/table"
	tabletext "github.com/jedib0t/go-pretty/v6/text"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/metrics"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/summary"
	"github.com/redhat-openshift-ecosystem/opct/internal/report"
	log "github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/sonobuoy/pkg/errlog"
)

type Input struct {
	archive         string
	archiveBase     string
	saveTo          string
	serverAddress   string
	serverSkip      bool
	embedData       bool
	saveOnly        bool
	verbose         bool
	json            bool
	skipBaselineAPI bool
	force           bool
}

var iconsCollor = map[string]string{
	"pass":   "✅",
	"passed": "✅",
	"fail":   "❌",
	"failed": "❌",
	"warn":   "⚠️", // there is a bug, the emoji is rendered breaking the table
	"alert":  "🚨",
}

var iconsBW = map[string]string{
	"pass":   "✔",
	"passed": "✔",
	"fail":   "✖",
	"failed": "✖",
	"warn":   "⚠",
	"alert":  "⚠",
}

func NewCmdReport() *cobra.Command {
	data := Input{}
	cmd := &cobra.Command{
		Use:   "report archive.tar.gz",
		Short: "Create a report from results.",
		Run: func(cmd *cobra.Command, args []string) {
			data.archive = args[0]
			checkFlags(&data)
			if err := processResult(&data); err != nil {
				errlog.LogError(errors.Wrapf(err, "could not process archive: %v", args[0]))
				os.Exit(1)
			}
		},
		Args: cobra.ExactArgs(1),
	}

	// TODO: Basline/Diff from CLI must be removed v0.6+ when the
	// report API is totally validated, introduced in v0.5.
	// report API is a serverless service storing CI results in S3, serving
	// summarized information through HTTP endpoint (CloudFront), it is consumed
	// in the filter pipeline while processing the report, preventing any additional
	// step from user to download a specific archive.
	cmd.Flags().StringVarP(
		&data.archiveBase, "baseline", "b", "",
		"[DEPRECATED] Baseline result archive file. Example: -b file.tar.gz",
	)
	cmd.Flags().StringVarP(
		&data.archiveBase, "diff", "d", "",
		"[DEPRECATED] Diff results from a baseline archive file. Example: --diff file.tar.gz",
	)

	cmd.Flags().StringVarP(
		&data.saveTo, "save-to", "s", "",
		"Extract and Save Results to disk. Example: -s ./results",
	)
	cmd.Flags().StringVar(
		&data.serverAddress, "server-address", "0.0.0.0:9090",
		"HTTP server address to serve files when --save-to is used. Example: --server-address 0.0.0.0:9090",
	)
	cmd.Flags().BoolVar(
		&data.serverSkip, "skip-server", false,
		"HTTP server address to serve files when --save-to is used. Example: --server-address 0.0.0.0:9090",
	)
	cmd.Flags().BoolVar(
		&data.embedData, "embed-data", false,
		"Force to embed the data into HTML report, allwoing the use of file protocol/CORS in the browser.",
	)
	cmd.Flags().BoolVar(
		&data.saveOnly, "save-only", false,
		"Save data and exit. Requires --save-to. Example: -s ./results --save-only",
	)
	cmd.Flags().BoolVarP(
		&data.verbose, "verbose", "v", false,
		"Show test details of test failures",
	)
	cmd.Flags().BoolVar(
		&data.json, "json", false,
		"Show report in json format",
	)
	cmd.Flags().BoolVar(
		&data.skipBaselineAPI, "skip-baseline-api", false,
		"Set to disable the BsaelineAPI call to get the baseline results injected in the failure filter pipeline.",
	)
	cmd.Flags().BoolVarP(
		&data.force, "force", "f", false,
		"Force to continue the execution, skipping deprecation warnings.",
	)
	return cmd
}

// checkFlags checks the flags and set the default values.
func checkFlags(input *Input) {
	if input.embedData {
		log.Warnf("--embed-data is set to true, forcing --server-skip to true.")
		input.serverSkip = true
	}
}

// processResult reads the artifacts and show it as an report format.
func processResult(input *Input) error {
	log.Println("Creating report...")
	timers := metrics.NewTimers()
	timers.Add("report-total")

	if input.skipBaselineAPI {
		log.Warnf("THIS IS NOT RECOMMENDED: detected flag --skip-baseline-api, setting OPCT_DISABLE_FILTER_BASELINE=1 to skip the failure filter in the pipeline")
		os.Setenv("OPCT_DISABLE_FILTER_BASELINE", "1")
	}

	// Show deprecation warnings when using --baseline.
	if input.archiveBase != "" {
		log.Warnf(`DEPRECATED: --baseline/--diff flag should not be used and will be removed soon.
Baseline are now discovered and applied to the filter pipeline automatically.
Please remove the --baseline/--diff flags from the command.
Additionally, if you want to skip the BaselineAPI filter, use --skip-baseline-api=true.`)
		if !input.force {
			log.Warnf("Aborting execution: --force flag is not set, set it if you want continue with warnings.")
			os.Exit(1)
		}
	}

	cs := summary.NewConsolidatedSummary(&summary.ConsolidatedSummaryInput{
		Verbose:     input.verbose,
		Timers:      timers,
		Archive:     input.archive,
		ArchiveBase: input.archiveBase,
		SaveTo:      input.saveTo,
	})

	log.Debug("Processing results")
	if err := cs.Process(); err != nil {
		return fmt.Errorf("error processing results: %v", err)
	}

	re := report.NewReportData(input.embedData)
	log.Debug("Processing report")
	if err := re.Populate(cs); err != nil {
		return fmt.Errorf("error populating report: %v", err)
	}

	// show report in CLI
	if err := showReportCLI(re, input.verbose); err != nil {
		return fmt.Errorf("error showing aggregated summary: %v", err)
	}

	if input.saveTo != "" {
		// TODO: ConsolidatedSummary should be migrated to SaveResults
		if err := cs.SaveResults(input.saveTo); err != nil {
			return fmt.Errorf("error saving consolidated summary results: %v", err)
		}
		timers.Add("report-total")
		if err := re.SaveResults(input.saveTo); err != nil {
			return fmt.Errorf("error saving report results: %v", err)
		}
		if input.saveOnly {
			os.Exit(0)
		}
	}

	// start http server to serve static report
	if input.saveTo != "" && !input.serverSkip {
		fs := http.FileServer(http.Dir(input.saveTo))
		// TODO: redirect home to the  opct-reporet.html (or rename to index.html) without
		// affecting the fileserver.
		http.Handle("/", fs)

		log.Infof("The report web UI can be accessed at http://%s", input.serverAddress)
		if err := http.ListenAndServe(input.serverAddress, nil); err != nil {
			log.Fatalf("Unable to start the report server at address %s: %v", input.serverAddress, err)
		}
	}
	if input.saveTo != "" && input.serverSkip {
		log.Infof("The report server is not enabled (--server-skip=true)., you'll need to navigate it locallly")
		log.Infof("To read the report open your browser and navigate to the path file://%s", input.saveTo)
		log.Infof("To get started open the report file://%s/index.html.", input.saveTo)
	}

	return nil
}

func showReportCLI(report *report.ReportData, verbose bool) error {
	if err := showReportAggregatedSummary(report); err != nil {
		return fmt.Errorf("error showing aggregated summary: %v", err)
	}
	if err := showProcessedSummary(report); err != nil {
		return fmt.Errorf("error showing processed summary: %v", err)
	}
	if err := showErrorDetails(report, verbose); err != nil {
		return fmt.Errorf("error showing error details: %v", err)
	}
	if err := showChecks(report); err != nil {
		return fmt.Errorf("error showing checks: %v", err)
	}
	return nil
}

func showReportAggregatedSummary(re *report.ReportData) error {
	baselineProcessed := re.Baseline != nil

	// Using go-table
	archive := filepath.Base(re.Summary.Tests.Archive)
	if re.Baseline != nil {
		archive = fmt.Sprintf("%s\n >> Diff from: %s", archive, filepath.Base(re.Summary.Tests.ArchiveDiff))
	}
	title := "OPCT Summary\n > Archive: " + archive

	// standalone results (provider)
	tbProv := table.NewWriter()
	tbProv.SetOutputMirror(os.Stdout)
	tbProv.SetStyle(table.StyleLight)
	tbProv.SetTitle(title)
	tbProv.AppendHeader(table.Row{"", "Provider"})

	// baseline results (provider+baseline)
	tbPBas := table.NewWriter()
	tbPBas.SetOutputMirror(os.Stdout)
	tbPBas.SetStyle(table.StyleLight)
	tbPBas.SetTitle(title)
	tbPBas.AppendHeader(table.Row{"", "Provider", "Baseline"})
	rowsPBas := []table.Row{}

	// Section: Cluster configuration
	joinPlatformType := func(infra *report.ReportInfra) string {
		tp := infra.PlatformType
		if tp == "External" {
			tp = fmt.Sprintf("%s (%s)", tp, infra.PlatformName)
		}
		return tp
	}
	rowsProv := []table.Row{{"Infrastructure:", ""}}
	rowsProv = append(rowsProv, table.Row{" PlatformType", joinPlatformType(re.Provider.Infra)})
	rowsProv = append(rowsProv, table.Row{" Name", re.Provider.Infra.Name})
	rowsProv = append(rowsProv, table.Row{" ClusterID", re.Provider.Version.OpenShift.ClusterID})
	rowsProv = append(rowsProv, table.Row{" Topology", re.Provider.Infra.Topology})
	rowsProv = append(rowsProv, table.Row{" ControlPlaneTopology", re.Provider.Infra.ControlPlaneTopology})
	rowsProv = append(rowsProv, table.Row{" API Server URL", re.Provider.Infra.APIServerURL})
	rowsProv = append(rowsProv, table.Row{" API Server URL (internal)", re.Provider.Infra.APIServerInternalURL})
	rowsProv = append(rowsProv, table.Row{" NetworkType", re.Provider.Infra.NetworkType})
	tbProv.AppendRows(rowsProv)
	tbProv.AppendSeparator()
	if baselineProcessed {
		rowsPBas = []table.Row{{"Infrastructure:", "", ""}}
		rowsPBas = append(rowsPBas, table.Row{" PlatformType", joinPlatformType(re.Provider.Infra), joinPlatformType(re.Baseline.Infra)})
		rowsPBas = append(rowsPBas, table.Row{" Name", re.Provider.Infra.Name, re.Baseline.Infra.Name})
		rowsPBas = append(rowsPBas, table.Row{" Topology", re.Provider.Infra.Topology, re.Baseline.Infra.Topology})
		rowsPBas = append(rowsPBas, table.Row{" ControlPlaneTopology", re.Provider.Infra.ControlPlaneTopology, re.Baseline.Infra.ControlPlaneTopology})
		rowsPBas = append(rowsPBas, table.Row{" API Server URL", re.Provider.Infra.APIServerURL, re.Baseline.Infra.APIServerURL})
		rowsPBas = append(rowsPBas, table.Row{" API Server URL (internal)", re.Provider.Infra.APIServerInternalURL, re.Baseline.Infra.APIServerInternalURL})
		rowsPBas = append(rowsPBas, table.Row{" NetworkType", re.Baseline.Infra.NetworkType})
		tbPBas.AppendRows(rowsPBas)
		tbPBas.AppendSeparator()
	}

	// Section: Cluster state
	rowsProv = []table.Row{{"Cluster Version:", ""}}
	rowsProv = append(rowsProv, table.Row{" Kubernetes", re.Provider.Version.Kubernetes})
	rowsProv = append(rowsProv, table.Row{" OpenShift", re.Provider.Version.OpenShift.Desired})
	rowsProv = append(rowsProv, table.Row{" Channel", re.Provider.Version.OpenShift.Channel})
	tbProv.AppendRows(rowsProv)
	tbProv.AppendSeparator()
	rowsProv = []table.Row{{"Cluster Status: ", re.Provider.Version.OpenShift.OverallStatus}}
	if re.Provider.Version.OpenShift.OverallStatus != "Available" {
		rowsProv = append(rowsProv, table.Row{" Reason", re.Provider.Version.OpenShift.OverallStatusReason})
		rowsProv = append(rowsProv, table.Row{" Message", re.Provider.Version.OpenShift.OverallStatusMessage})
	}
	rowsProv = append(rowsProv, table.Row{"Cluster Status/Conditions:", ""})
	rowsProv = append(rowsProv, table.Row{" Available", re.Provider.Version.OpenShift.CondAvailable})
	rowsProv = append(rowsProv, table.Row{" Failing", re.Provider.Version.OpenShift.CondFailing})
	rowsProv = append(rowsProv, table.Row{" Progressing (Update)", re.Provider.Version.OpenShift.CondProgressing})
	rowsProv = append(rowsProv, table.Row{" RetrievedUpdates", re.Provider.Version.OpenShift.CondRetrievedUpdates})
	rowsProv = append(rowsProv, table.Row{" EnabledCapabilities", re.Provider.Version.OpenShift.CondImplicitlyEnabledCapabilities})
	rowsProv = append(rowsProv, table.Row{" ReleaseAccepted", re.Provider.Version.OpenShift.CondReleaseAccepted})
	tbProv.AppendRows(rowsProv)
	tbProv.AppendSeparator()

	if baselineProcessed {
		rowsPBas = []table.Row{{"Cluster Version:", "", ""}}
		rowsPBas = append(rowsPBas, table.Row{" Kubernetes", re.Provider.Version.Kubernetes, re.Baseline.Version.Kubernetes})
		rowsPBas = append(rowsPBas, table.Row{" OpenShift", re.Provider.Version.OpenShift.Desired, re.Baseline.Version.OpenShift.Desired})
		rowsPBas = append(rowsPBas, table.Row{" Channel", re.Provider.Version.OpenShift.Channel, re.Baseline.Version.OpenShift.Channel})
		tbPBas.AppendRows(rowsPBas)
		tbPBas.AppendSeparator()
		rowsPBas = []table.Row{{"Cluster Status: ", re.Provider.Version.OpenShift.OverallStatus, re.Baseline.Version.OpenShift.OverallStatus}}
		if re.Provider.Version.OpenShift.OverallStatus != "Available" {
			rowsPBas = append(rowsPBas, table.Row{" Reason", re.Provider.Version.OpenShift.OverallStatusReason, re.Baseline.Version.OpenShift.OverallStatusReason})
			rowsPBas = append(rowsPBas, table.Row{" Message", re.Provider.Version.OpenShift.OverallStatusMessage, re.Baseline.Version.OpenShift.OverallStatusMessage})
		}
		rowsPBas = append(rowsPBas, table.Row{"Cluster Status/Conditions:", "", ""})
		rowsPBas = append(rowsPBas, table.Row{" Available", re.Provider.Version.OpenShift.CondAvailable, re.Baseline.Version.OpenShift.CondAvailable})
		rowsPBas = append(rowsPBas, table.Row{" Failing", re.Provider.Version.OpenShift.CondFailing, re.Baseline.Version.OpenShift.CondFailing})
		rowsPBas = append(rowsPBas, table.Row{" Progressing (Update)", re.Provider.Version.OpenShift.CondProgressing, re.Baseline.Version.OpenShift.CondProgressing})
		rowsPBas = append(rowsPBas, table.Row{" RetrievedUpdates", re.Provider.Version.OpenShift.CondRetrievedUpdates, re.Baseline.Version.OpenShift.CondRetrievedUpdates})
		rowsPBas = append(rowsPBas, table.Row{" EnabledCapabilities", re.Provider.Version.OpenShift.CondImplicitlyEnabledCapabilities, re.Baseline.Version.OpenShift.CondImplicitlyEnabledCapabilities})
		rowsPBas = append(rowsPBas, table.Row{" ReleaseAccepted", re.Provider.Version.OpenShift.CondReleaseAccepted, re.Baseline.Version.OpenShift.CondReleaseAccepted})
		tbPBas.AppendRows(rowsPBas)
		tbPBas.AppendSeparator()
	}

	// Section: Environment state
	rowsProv = []table.Row{{"Plugin summary:", "Status [Total/Passed/Failed/Skipped] (timeout)"}}
	if baselineProcessed {
		rowsPBas = []table.Row{{"Plugin summary:", "Status [Total/Passed/Failed/Skipped] (timeout)", ""}}
	}

	showPluginSummary := func(w *tabwriter.Writer, pluginName string) {
		if _, ok := re.Provider.Plugins[pluginName]; !ok {
			errlog.LogError(errors.New(fmt.Sprintf("Unable to load plugin %s", pluginName)))
		}
		plK8S := re.Provider.Plugins[pluginName]
		name := fmt.Sprintf(" %s", plK8S.Name)
		stat := plK8S.Stat
		pOCPPluginRes := fmt.Sprintf("%s [%d/%d/%d/%d] (%d)", stat.Status, stat.Total, stat.Passed, stat.Failed, stat.Skipped, stat.Timeout)
		rowsProv = append(rowsProv, table.Row{name, pOCPPluginRes})
		if baselineProcessed {
			plK8S = re.Baseline.Plugins[pluginName]
			stat := plK8S.Stat
			bOCPPluginRes := fmt.Sprintf("%s [%d/%d/%d/%d] (%d)", stat.Status, stat.Total, stat.Passed, stat.Failed, stat.Skipped, stat.Timeout)
			// fmt.Fprintf(tbWriter, " - %s\t: %s\t: %s\n", name, pOCPPluginRes, bOCPPluginRes)
			rowsPBas = append(rowsPBas, table.Row{name, pOCPPluginRes, bOCPPluginRes})
		}
	}

	showPluginSummary(nil, plugin.PluginNameKubernetesConformance)
	showPluginSummary(nil, plugin.PluginNameOpenShiftConformance)
	showPluginSummary(nil, plugin.PluginNameOpenShiftUpgrade)

	tbProv.AppendRows(rowsProv)
	tbProv.AppendSeparator()
	rowsProv = []table.Row{{"Env health summary:", "[A=True/P=True/D=True]"}}
	if baselineProcessed {
		tbPBas.AppendRows(rowsPBas)
		tbPBas.AppendSeparator()
		rowsPBas = []table.Row{{"Env health summary:", "[A=True/P=True/D=True]", ""}}
	}

	pOCPCO := re.Provider.ClusterOperators
	rowsProv = append(rowsProv, table.Row{
		" Cluster Operators",
		fmt.Sprintf("[%d/%d/%d]", pOCPCO.CountAvailable, pOCPCO.CountProgressing, pOCPCO.CountDegraded),
	})
	if baselineProcessed {
		bOCPCO := re.Baseline.ClusterOperators
		rowsPBas = append(rowsPBas, table.Row{
			" Cluster Operators",
			fmt.Sprintf("[%d/%d/%d]", pOCPCO.CountAvailable, pOCPCO.CountProgressing, pOCPCO.CountDegraded),
			fmt.Sprintf("[%d/%d/%d]", bOCPCO.CountAvailable, bOCPCO.CountProgressing, bOCPCO.CountDegraded),
		})
	}

	// Show Nodes Health info collected by Sonobuoy
	pNhMessage := fmt.Sprintf("%d/%d %s", re.Provider.ClusterHealth.NodeHealthy, re.Provider.ClusterHealth.NodeHealthTotal, "")
	if re.Provider.ClusterHealth.NodeHealthTotal != 0 {
		pNhMessage = fmt.Sprintf("%s (%.2f%%)", pNhMessage, re.Provider.ClusterHealth.NodeHealthPerc)
	}

	rowsProv = append(rowsProv, table.Row{" Node health", pNhMessage})
	if baselineProcessed {
		bNhMessage := fmt.Sprintf("%d/%d %s", re.Baseline.ClusterHealth.NodeHealthy, re.Baseline.ClusterHealth.NodeHealthTotal, "")
		if re.Baseline.ClusterHealth.NodeHealthTotal != 0 {
			bNhMessage = fmt.Sprintf("%s (%.2f%%)", bNhMessage, re.Baseline.ClusterHealth.NodeHealthPerc)
		}
		rowsPBas = append(rowsPBas, table.Row{" Node health", pNhMessage, bNhMessage})
	}

	// Show Pods Health info collected by Sonobuoy
	pPodsHealthMsg := ""
	bPodsHealthMsg := ""
	phTotal := ""

	if re.Provider.ClusterHealth.PodHealthTotal != 0 {
		phTotal = fmt.Sprintf(" (%.2f%%)", re.Provider.ClusterHealth.PodHealthPerc)
	}
	pPodsHealthMsg = fmt.Sprintf("%d/%d %s", re.Provider.ClusterHealth.PodHealthy, re.Provider.ClusterHealth.PodHealthTotal, phTotal)
	rowsProv = append(rowsProv, table.Row{" Pods health", pPodsHealthMsg})
	if baselineProcessed {
		phTotal := ""
		if re.Baseline.ClusterHealth.PodHealthTotal != 0 {
			phTotal = fmt.Sprintf(" (%.2f%%)", re.Baseline.ClusterHealth.PodHealthPerc)
		}
		bPodsHealthMsg = fmt.Sprintf("%d/%d %s", re.Baseline.ClusterHealth.PodHealthy, re.Baseline.ClusterHealth.PodHealthTotal, phTotal)
		rowsPBas = append(rowsPBas, table.Row{" Pods health", pPodsHealthMsg, bPodsHealthMsg})
	}

	// Section: Test count by suite
	tbProv.AppendRows(rowsProv)
	tbProv.AppendSeparator()
	rowsProv = []table.Row{{"Test count by suite:", ""}}
	if baselineProcessed {
		tbPBas.AppendRows(rowsPBas)
		tbPBas.AppendSeparator()
		rowsPBas = []table.Row{{"Test count by suite:", "", ""}}
	}

	checkEmpty := func(counter int) string {
		if counter == 0 {
			return "(FAIL)"
		}
		return ""
	}
	rowsProv = append(rowsProv, table.Row{
		summary.SuiteNameKubernetesConformance,
		fmt.Sprintf("%d %s",
			re.Provider.Plugins[plugin.PluginNameKubernetesConformance].Suite.Count,
			checkEmpty(re.Provider.Plugins[plugin.PluginNameKubernetesConformance].Suite.Count),
		),
	})
	rowsProv = append(rowsProv, table.Row{
		summary.SuiteNameOpenshiftConformance,
		fmt.Sprintf("%d %s",
			re.Provider.Plugins[plugin.PluginNameOpenShiftConformance].Suite.Count,
			checkEmpty(re.Provider.Plugins[plugin.PluginNameOpenShiftConformance].Suite.Count),
		),
	})
	if baselineProcessed {
		p := re.Baseline.Plugins[plugin.PluginNameKubernetesConformance]
		if p != nil && p.Suite != nil {
			rowsPBas = append(rowsPBas, table.Row{
				summary.SuiteNameKubernetesConformance,
				fmt.Sprintf("%d %s",
					re.Provider.Plugins[plugin.PluginNameKubernetesConformance].Suite.Count,
					checkEmpty(re.Provider.Plugins[plugin.PluginNameKubernetesConformance].Suite.Count),
				),
				fmt.Sprintf("%d %s", p.Suite.Count, checkEmpty(p.Suite.Count)),
			})
		}
		p = re.Baseline.Plugins[plugin.PluginNameOpenShiftConformance]
		if p != nil && p.Suite != nil {
			rowsPBas = append(rowsPBas, table.Row{
				summary.SuiteNameOpenshiftConformance,
				fmt.Sprintf("%d %s",
					re.Provider.Plugins[plugin.PluginNameOpenShiftConformance].Suite.Count,
					checkEmpty(re.Provider.Plugins[plugin.PluginNameOpenShiftConformance].Suite.Count),
				),
				fmt.Sprintf("%d %s", p.Suite.Count, checkEmpty(p.Suite.Count)),
			})
		}
	}

	// Decide which table to show.
	if baselineProcessed {
		// Table done (provider + baseline)
		tbPBas.AppendRows(rowsPBas)
		tbPBas.Render()
	} else {
		// Table done (provider)
		tbProv.AppendRows(rowsProv)
		tbProv.Render()
	}

	// Section: Failed pods counter (using old table version [tabwritter])
	newLineWithTab := "\t\t\n"
	tbWriter := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
	fmt.Fprint(tbWriter, newLineWithTab)
	if len(re.Provider.ClusterHealth.PodHealthDetails) > 0 {
		fmt.Fprintf(tbWriter, " Failed pods:\n")
		fmt.Fprintf(tbWriter, "  %s/%s\t%s\t%s\t%s\t%s\n", "Namespace", "PodName", "Healthy", "Ready", "Reason", "Message")
		for _, podDetails := range re.Provider.ClusterHealth.PodHealthDetails {
			fmt.Fprintf(tbWriter, "  %s/%s\t%t\t%s\t%s\t%s\n", podDetails.Namespace, podDetails.Name, podDetails.Healthy, podDetails.Ready, podDetails.Reason, podDetails.Message)
		}
	}
	tbWriter.Flush()

	return nil
}

func showProcessedSummary(re *report.ReportData) error {
	fmt.Printf("\n=> Processed Summary <=\n")
	fmt.Printf("==> Result Summary by test suite:\n")
	bProcessed := re.Provider.HasValidBaseline
	plugins := re.Provider.GetPlugins()
	sort.Strings(plugins)
	for _, pluginName := range plugins {
		showSummaryPlugin(re.Provider, pluginName, bProcessed)
	}
	return nil
}

func showSummaryPlugin(re *report.ReportResult, pluginName string, bProcessed bool) {
	if re.Plugins[pluginName] == nil {
		log.Errorf("unable to get plugin %s", pluginName)
		return
	}
	p := re.Plugins[pluginName]
	if p.Stat == nil {
		log.Errorf("unable to get stat for plugin %s", pluginName)
		return
	}

	tb := table.NewWriter()
	tb.SetOutputMirror(os.Stdout)
	tb.SetStyle(table.StyleLight)
	title := fmt.Sprintf("%s:", p.Name)
	titleIcon := ""
	tb.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, WidthMin: 25, WidthMax: 25},
		{Number: 2, WidthMin: 13, WidthMax: 13},
	})
	rows := []table.Row{}

	renderTable := func() {
		title = fmt.Sprintf("%s %s", title, titleIcon)
		tb.SetTitle(title)
		tb.Render()
	}

	stat := p.Stat
	rows = append(rows, table.Row{"Total tests", stat.Total})
	rows = append(rows, table.Row{"Passed", stat.Passed})
	rows = append(rows, table.Row{"Failed", stat.Failed})
	rows = append(rows, table.Row{"Timeout", stat.Timeout})
	rows = append(rows, table.Row{"Skipped", stat.Skipped})
	titleIcon = iconsCollor[stat.Status]

	if p.Name == plugin.PluginNameOpenShiftUpgrade || p.Name == plugin.PluginNameArtifactsCollector {
		rows = append(rows, table.Row{"Result Job", stat.Status})
		tb.AppendRows(rows)
		renderTable()
		return
	}
	rows = append(rows, table.Row{"Filter Failed Suite", plugin.UtilsCalcPercStr(stat.FilterSuite, stat.Total)})
	rows = append(rows, table.Row{"Filter Failed KF", plugin.UtilsCalcPercStr(stat.Filter5Failures, stat.Total)})
	rows = append(rows, table.Row{"Filter Replay", plugin.UtilsCalcPercStr(stat.Filter6Failures, stat.Total)})
	rows = append(rows, table.Row{"Filter Failed Baseline", plugin.UtilsCalcPercStr(stat.FilterBaseline, stat.Total)})
	rows = append(rows, table.Row{"Filter Failed Priority", plugin.UtilsCalcPercStr(stat.FilterFailedPrio, stat.Total)})
	rows = append(rows, table.Row{"Filter Failed API", plugin.UtilsCalcPercStr(stat.FilterFailedAPI, stat.Total)})
	rows = append(rows, table.Row{"Failures (Priotity)", plugin.UtilsCalcPercStr(stat.FilterFailures, stat.Total)})

	// TODO(mtulio): review suites provides better signal.
	// The final results for non-kubernetes conformance will be hidden (pass|fail) for a while for those reasons:
	// - OPCT was created to provide feeaback of conformance results, not a passing binary value. The numbers should be interpreted individually
	// - Conformance results could have flakes or runtime failures which need to be investigated by executor
	// - Force user/executor to review the results, and not only the summary.
	// That behavior is aligned with BU: we expect kubernetes conformance passes in all providers, the reviewer
	// must set this as a target in the review process.
	// UPDATED(mtulio): OPCT is providing signals for conformance suites. The openshift-validated/conformance
	// passing after filters means the baseline has common failures, which needs to be investigated in the future
	// for non-providers - because there is a big chance to be related with the environment or platform-wide issue/bug.
	// Leaving it commmented and providing a 'processed' result for openshift-conformance too.
	// if p.Name != plugin.PluginNameKubernetesConformance {
	// 	rows = append(rows, table.Row{"Result Job", stat.Status})
	// 	tb.AppendRows(rows)
	// 	renderTable()
	// 	return
	// }

	// checking for runtime failures
	runtimeFailed := false
	if stat.Total == stat.Failed {
		runtimeFailed = true
	}

	// rewrite the original status when pass on all filters and not failed on runtime
	status := stat.Status
	if (stat.FilterFailures == 0) && !runtimeFailed {
		status = "passed"
	}

	rows = append(rows, table.Row{"Result - Job", stat.Status})
	rows = append(rows, table.Row{"Result - Processed", status})
	tb.AppendRows(rows)
	titleIcon = iconsCollor[status]
	if p.Name == plugin.PluginNameConformanceReplay && status != "passed" {
		titleIcon = iconsBW["warn"]
	}
	renderTable()
}

// showErrorDetails show details of failres for each plugin.
func showErrorDetails(re *report.ReportData, verbose bool) error {
	fmt.Printf("\n==> Result details by conformance plugins: \n")

	showErrorDetailPlugin(re.Provider.Plugins[plugin.PluginNameKubernetesConformance], verbose)
	showErrorDetailPlugin(re.Provider.Plugins[plugin.PluginNameOpenShiftConformance], verbose)

	return nil
}

// showErrorDetailPlugin Show failed e2e tests by filter, when verbose each filter will be shown.
func showErrorDetailPlugin(p *report.ReportPlugin, verbose bool) {
	if p == nil {
		errlog.LogError(errors.New("unable to get plugin"))
		return
	}
	fmt.Printf("==> %s - test failures:\n", p.Name)

	// Plugin failures table - setup
	st := table.StyleLight
	st.Options.SeparateRows = true

	// Build table utility
	defaultHeaderRow := table.Row{"#Err", "#Flake", "%Flake", "State", "Test Name"}
	buildTable := func(title string, header table.Row) table.Writer {
		tb := table.NewWriter()
		tb.SetOutputMirror(os.Stdout)
		tb.SetStyle(st)
		tb.SetTitle(title)
		tb.AppendHeader(header)
		tb.SetColumnConfigs([]table.ColumnConfig{
			{Number: 5, AlignHeader: tabletext.AlignCenter, WidthMax: 67},
		})
		return tb
	}
	populateTable := func(tb table.Writer, failures []*report.ReportTestFailure, skipFlake bool) {
		for _, failure := range failures {
			test, ok := p.Tests[failure.Name]
			if !ok {
				errlog.LogError(errors.New(fmt.Sprintf("unable to get test %s", failure.Name)))
				continue
			}
			errCount := 0
			if _, ok := test.ErrorCounters["total"]; ok {
				errCount = test.ErrorCounters["total"]
			}
			if skipFlake {
				tb.AppendRow(table.Row{errCount, test.Name})
				continue
			}
			if test.Flake == nil {
				tb.AppendRow(table.Row{errCount, "--", "--", test.State, test.Name})
				continue
			}
			tb.AppendRow(table.Row{errCount, test.Flake.CurrentFlakes, fmt.Sprintf("%.2f", test.Flake.CurrentFlakePerc), test.State, test.Name})
		}
		tb.Render()
	}

	// Table for Priority
	if len(p.FailedFiltered) > 0 {
		tb := table.NewWriter()
		tb.SetOutputMirror(os.Stdout)
		tb.SetStyle(st)
		tb.AppendHeader(table.Row{"#Err", "Test Name"})
		tb.AppendFooter(table.Row{"", p.TagsFiltered})
		tb.SetColumnConfigs([]table.ColumnConfig{
			{Number: 2, AlignHeader: tabletext.AlignCenter, WidthMax: 100},
		})
		title := fmt.Sprintf("==> %s \n%s ACTION REQUIRED: Failed tests to review", p.Name, iconsCollor["alert"])
		tb.SetTitle(title)
		populateTable(tb, p.FailedFiltered, true)
	}

	// Table for Flakes
	if len(p.FailedFilter3) > 0 {
		filterName := "FlakeAPI"
		title := fmt.Sprintf("\n==> %s\n Failed tests excluded in %q filter (%d)", p.Name, filterName, len(p.FailedFilter3))
		tb := buildTable(title, defaultHeaderRow)
		populateTable(tb, p.FailedFilter3, false)
	}

	if verbose {
		fmt.Printf("\n Show failures by filter (verbose mode): %v\n", verbose)
		skipFlake := false
		titlePrefix := fmt.Sprintf("==> %s\n Failed tests excluded in", p.Name)
		// Show tests removed in filter: Replay
		if len(p.FailedFilter6) > 0 {
			filterName := "Replay"
			title := fmt.Sprintf("%s %q filter (%d)", titlePrefix, filterName, len(p.FailedFilter6))
			tb := buildTable(title, defaultHeaderRow)
			populateTable(tb, p.FailedFilter6, skipFlake)
		}

		// Show tests removed in filter: KnownFailures
		if len(p.FailedFilter5) > 0 {
			filterName := "KnownFailures"
			title := fmt.Sprintf("%s %q filter (%d)", titlePrefix, filterName, len(p.FailedFilter5))
			tb := buildTable(title, defaultHeaderRow)
			populateTable(tb, p.FailedFilter5, skipFlake)
		}

		// Show tests removed in filter: BaselineArchive
		if len(p.FailedFilter2) > 0 {
			filterName := "BaselineArchive"
			title := fmt.Sprintf("%s %q filter (%d)", titlePrefix, filterName, len(p.FailedFilter2))
			tb := buildTable(title, defaultHeaderRow)
			populateTable(tb, p.FailedFilter2, skipFlake)
		}

		// Show tests removed in filter: SuiteOnly
		if len(p.FailedFilter1) > 0 {
			filterName := "SuiteOnly"
			title := fmt.Sprintf("%s %q filter (%d)", titlePrefix, filterName, len(p.FailedFilter1))
			tb := buildTable(title, defaultHeaderRow)
			populateTable(tb, p.FailedFilter1, skipFlake)
		}
	}
}

// showChecks show the checks results / final report.
func showChecks(re *report.ReportData) error {
	rowsFailures := []table.Row{}
	rowsWarns := []table.Row{}
	rowsPass := []table.Row{}
	rowSkip := []table.Row{}

	fmt.Printf("\n\n")
	tb := table.NewWriter()
	tb.SetOutputMirror(os.Stdout)
	tb.SetStyle(table.StyleLight)
	tb.AppendHeader(table.Row{"ID", "#", "Result", "Check name", "Target", "Current"})
	tb.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AlignHeader: tabletext.AlignCenter},
		{Number: 2, AlignHeader: tabletext.AlignCenter, Align: tabletext.AlignCenter},
		{Number: 3, AlignHeader: tabletext.AlignCenter, Align: tabletext.AlignCenter},
		{Number: 4, AlignHeader: tabletext.AlignCenter, AlignFooter: tabletext.AlignCenter},
		{Number: 5, AlignHeader: tabletext.AlignCenter},
		{Number: 6, AlignHeader: tabletext.AlignCenter},
	})

	allChecks := append([]*report.SLOOutput{}, re.Checks.Fail...)
	allChecks = append(allChecks, re.Checks.Warn...)
	allChecks = append(allChecks, re.Checks.Pass...)
	allChecks = append(allChecks, re.Checks.Skip...)
	for _, check := range re.Checks.Fail {
		rowsFailures = append(rowsFailures, table.Row{
			check.ID, iconsCollor[check.SLOResult], check.SLOResult, check.SLO, check.SLITarget, check.SLIActual,
		})
	}
	for _, check := range re.Checks.Warn {
		rowsWarns = append(rowsWarns, table.Row{
			check.ID, iconsBW[check.SLOResult], check.SLOResult, check.SLO, check.SLITarget, check.SLIActual,
		})
	}
	for _, check := range re.Checks.Pass {
		rowsPass = append(rowsPass, table.Row{
			check.ID, iconsBW[check.SLOResult], check.SLOResult, check.SLO, check.SLITarget, check.SLIActual,
		})
	}
	for _, check := range re.Checks.Skip {
		rowSkip = append(rowSkip, table.Row{
			check.ID, iconsBW["pass"], check.SLOResult, check.SLO, check.SLITarget, check.SLIActual,
		})
	}

	if len(rowsFailures) > 0 {
		tb.AppendRows(rowsFailures)
		tb.AppendSeparator()
	}
	if len(rowsWarns) > 0 {
		tb.AppendRows(rowsWarns)
		tb.AppendSeparator()
	}
	if len(rowsPass) > 0 {
		tb.AppendRows(rowsPass)
		tb.AppendSeparator()
	}
	if len(rowSkip) > 0 {
		tb.AppendRows(rowSkip)
	}

	total := len(allChecks)
	summary := fmt.Sprintf("Total: %d, Failed: %d (%.2f%%), Warn: %d (%.2f%%), Pass: %d (%.2f%%), Skip: %d (%.2f%%)", total,
		len(re.Checks.Fail), (float64(len(re.Checks.Fail))/float64(total))*100,
		len(re.Checks.Warn), (float64(len(re.Checks.Warn))/float64(total))*100,
		len(re.Checks.Pass), (float64(len(re.Checks.Pass))/float64(total))*100,
		len(re.Checks.Skip), (float64(len(re.Checks.Skip))/float64(total))*100,
	)
	tb.AppendFooter(table.Row{"", "", "", summary, "", ""})

	title := "Validation checks / Results"
	// Create a alert message when there are check failures.
	if len(rowsFailures) > 0 {
		alert := fmt.Sprintf(
			"\t %s %s IMMEDIATE ACTION: %d Check(s) failed. Review it individually, fix and collect new results %s %s",
			iconsCollor["alert"], iconsCollor["alert"], len(re.Checks.Fail), iconsCollor["alert"], iconsCollor["alert"])
		title = fmt.Sprintf("%s\n%s", title, alert)
	}
	tb.SetTitle(title)
	tb.Render()

	return nil
}
