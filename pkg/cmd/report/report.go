package report

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"text/tabwriter"

	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/metrics"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/plugin"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/report"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/summary"
	log "github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/sonobuoy/pkg/errlog"
)

type Input struct {
	archive       string
	archiveBase   string
	saveTo        string
	serverAddress string
	serverSkip    bool
	embedData     bool
	saveOnly      bool
	verbose       bool
	json          bool
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

	cmd.Flags().StringVarP(
		&data.archiveBase, "baseline", "b", "",
		"[DEPRECATED] Baseline result archive file. Example: -b file.tar.gz",
	)
	cmd.Flags().StringVarP(
		&data.archiveBase, "diff", "d", "",
		"Diff results from a baseline archive file. Example: --diff file.tar.gz",
	)
	cmd.Flags().StringVarP(
		&data.saveTo, "save-to", "s", "",
		"Extract and Save Results to disk. Example: -s ./results",
	)
	cmd.Flags().StringVarP(
		&data.serverAddress, "server-address", "", "0.0.0.0:9090",
		"HTTP server address to serve files when --save-to is used. Example: --server-address 0.0.0.0:9090",
	)
	cmd.Flags().BoolVarP(
		&data.serverSkip, "server-skip", "", false,
		"HTTP server address to serve files when --save-to is used. Example: --server-address 0.0.0.0:9090",
	)
	cmd.Flags().BoolVarP(
		&data.embedData, "embed-data", "", false,
		"Force to embed the data into HTML report, allwoing the use of file protocol/CORS in the browser.",
	)
	cmd.Flags().BoolVarP(
		&data.saveOnly, "save-only", "", false,
		"Save data and exit. Requires --save-to. Example: -s ./results --save-only",
	)
	cmd.Flags().BoolVarP(
		&data.verbose, "verbose", "v", false,
		"Show test details of test failures",
	)
	cmd.Flags().BoolVarP(
		&data.json, "json", "", false,
		"Show report in json format",
	)

	return cmd
}

// checkFlags
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

	report := &report.Report{
		Setup: &report.ReportSetup{
			Frontend: &report.ReportSetupFrontend{
				EmbedData: input.embedData,
			},
		},
	}
	cs := summary.ConsolidatedSummary{
		Verbose: input.verbose,
		Timers:  timers,
		Provider: &summary.ResultSummary{
			Name:      summary.ResultSourceNameProvider,
			Archive:   input.archive,
			OpenShift: &summary.OpenShiftSummary{},
			Sonobuoy:  summary.NewSonobuoySummary(),
			Suites: &summary.OpenshiftTestsSuites{
				OpenshiftConformance:  &summary.OpenshiftTestsSuite{Name: "openshiftConformance"},
				KubernetesConformance: &summary.OpenshiftTestsSuite{Name: "kubernetesConformance"},
			},
			SavePath: input.saveTo,
		},
		Baseline: &summary.ResultSummary{
			Name:      summary.ResultSourceNameBaseline,
			Archive:   input.archiveBase,
			OpenShift: &summary.OpenShiftSummary{},
			Sonobuoy:  summary.NewSonobuoySummary(),
			Suites: &summary.OpenshiftTestsSuites{
				OpenshiftConformance:  &summary.OpenshiftTestsSuite{Name: "openshiftConformance"},
				KubernetesConformance: &summary.OpenshiftTestsSuite{Name: "kubernetesConformance"},
			},
		},
	}

	log.Debug("Processing results")
	if err := cs.Process(); err != nil {
		return err
	}

	log.Debug("Processing report")
	if err := report.Populate(&cs); err != nil {
		return err
	}

	if input.json {
		timers.Add("report-total")
		resReport, err := report.ShowJSON()
		if err != nil {
			return err
		}
		fmt.Println(resReport)
		os.Exit(0)
	}

	if input.saveTo != "" {
		// TODO: ConsolidatedSummary should be migrated to SaveResults
		if err := cs.SaveResults(input.saveTo); err != nil {
			return err
		}
		timers.Add("report-total")
		if err := report.SaveResults(input.saveTo); err != nil {
			return err
		}
		if input.saveOnly {
			os.Exit(0)
		}
	}

	if err := showReportAggregatedSummary(report); err != nil {
		return err
	}

	if err := showProcessedSummary(report); err != nil {
		return err
	}

	if err := showErrorDetails(report, input.verbose); err != nil {
		return err
	}

	if err := showChecks(report); err != nil {
		return err
	}

	// run http server to serve static report
	if input.saveTo != "" && !input.serverSkip {
		fs := http.FileServer(http.Dir(input.saveTo))
		// TODO: redirect home to the  opct-reporet.html (or rename to index.html) without
		// affecting the fileserver.
		http.Handle("/", fs)

		log.Debugf("Listening on %s...", input.serverAddress)
		log.Infof("The report server is available in http://%s, open your browser and navigate to results.", input.serverAddress)
		log.Infof("To get started open the report http://%s/opct-report.html.", input.serverAddress)
		err := http.ListenAndServe(input.serverAddress, nil)
		if err != nil {
			log.Fatalf("Unable to start the report server at address %s: %v", input.serverAddress, err)
		}
	}
	if input.saveTo != "" && input.serverSkip {
		log.Infof("The report server is not enabled (--server-skip=true)., you'll need to navigate it locallly")
		log.Infof("To read the report open your browser and navigate to the path file://%s", input.saveTo)
		log.Infof("To get started open the report file://%s/opct-report.html.", input.saveTo)
	}

	return nil
}

func showReportAggregatedSummary(re *report.Report) error {
	fmt.Printf("\n> OPCT Summary <\n\n")

	baselineProcessed := re.Baseline != nil

	newLineWithTab := "\t\t\n"
	tbWriter := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)

	if baselineProcessed {
		fmt.Fprintf(tbWriter, " Cluster Version:\n")
		fmt.Fprintf(tbWriter, " - Kubernetes\t: %s\t: %s\n", re.Provider.Version.Kubernetes, re.Baseline.Version.Kubernetes)
		fmt.Fprintf(tbWriter, " - OpenShift\t: %s\t: %s\n", re.Provider.Version.OpenShift.Desired, re.Baseline.Version.OpenShift.Desired)
		// fmt.Fprintf(tbWriter, " - Cluster Update Progressing\t: %s\t: %s\n", re.Provider.Version.OpenShiftUpdProg, re.Baseline.Version.OpenShiftUpdProg)
		fmt.Fprintf(tbWriter, " - OpenShift (Previous)\t: %s\t: %s\n", re.Provider.Version.OpenShift.Previous, re.Baseline.Version.OpenShift.Previous)
		fmt.Fprintf(tbWriter, " - Channel\t: %s\t: %s\n", re.Provider.Version.OpenShift.Channel, re.Baseline.Version.OpenShift.Channel)
	} else {
		fmt.Fprintf(tbWriter, " Cluster Version:\n")
		fmt.Fprintf(tbWriter, " - Kubernetes\t: %s\n", re.Provider.Version.Kubernetes)
		fmt.Fprintf(tbWriter, " - OpenShift\t: %s\n", re.Provider.Version.OpenShift.Desired)

		// fmt.Fprintf(tbWriter, " - OpenShift Previous\t: %s\n", re.Provider.Version.OpenShift.Previous)
		fmt.Fprintf(tbWriter, " - Channel\t: %s\n", re.Provider.Version.OpenShift.Channel)
		fmt.Fprintf(tbWriter, " Cluster Status\t: %s\n", re.Provider.Version.OpenShift.OverallStatus)
		if re.Provider.Version.OpenShift.OverallStatus != "Available" {
			fmt.Fprintf(tbWriter, " - Reason\t: %s\n", re.Provider.Version.OpenShift.OverallStatusReason)
			fmt.Fprintf(tbWriter, " - Message\t: %s\n", re.Provider.Version.OpenShift.OverallStatusMessage)
		}
		fmt.Fprintf(tbWriter, " Cluster Status/Conditions:\n")
		fmt.Fprintf(tbWriter, " - Available\t: %s\n", re.Provider.Version.OpenShift.CondAvailable)
		fmt.Fprintf(tbWriter, " - Failing\t: %s\n", re.Provider.Version.OpenShift.CondFailing)
		fmt.Fprintf(tbWriter, " - Progressing (Update)\t: %s\n", re.Provider.Version.OpenShift.CondProgressing)
		fmt.Fprintf(tbWriter, " - RetrievedUpdates\t: %s\n", re.Provider.Version.OpenShift.CondRetrievedUpdates)
		fmt.Fprintf(tbWriter, " - EnabledCapabilities\t: %s\n", re.Provider.Version.OpenShift.CondImplicitlyEnabledCapabilities)
		fmt.Fprintf(tbWriter, " - ReleaseAccepted\t: %s\n", re.Provider.Version.OpenShift.CondReleaseAccepted)
	}

	fmt.Fprint(tbWriter, newLineWithTab)
	joinPlatformType := func(infra *report.ReportInfra) string {
		tp := infra.PlatformType
		if tp == "External" {
			tp = fmt.Sprintf("%s (%s)", tp, infra.PlatformName)
		}
		return tp
	}
	if baselineProcessed {
		fmt.Fprintf(tbWriter, " Infrastructure:\t\t\n")
		fmt.Fprintf(tbWriter, " - PlatformType\t: %s\t: %s\n", joinPlatformType(re.Provider.Infra), joinPlatformType(re.Baseline.Infra))
		fmt.Fprintf(tbWriter, " - Name\t: %s\t: %s\n", re.Provider.Infra.Name, re.Baseline.Infra.Name)
		fmt.Fprintf(tbWriter, " - Topology\t: %s\t: %s\n", re.Provider.Infra.Topology, re.Baseline.Infra.Topology)
		fmt.Fprintf(tbWriter, " - ControlPlaneTopology\t: %s\t: %s\n", re.Provider.Infra.ControlPlaneTopology, re.Baseline.Infra.ControlPlaneTopology)
		fmt.Fprintf(tbWriter, " - API Server URL\t: %s\t: %s\n", re.Provider.Infra.APIServerURL, re.Baseline.Infra.APIServerURL)
		fmt.Fprintf(tbWriter, " - API Server URL (internal)\t: %s\t: %s\n", re.Provider.Infra.APIServerInternalURL, re.Baseline.Infra.APIServerInternalURL)
	} else {
		fmt.Fprintf(tbWriter, " Infrastructure:\t\n")
		fmt.Fprintf(tbWriter, " - PlatformType\t: %s\n", joinPlatformType(re.Provider.Infra))
		fmt.Fprintf(tbWriter, " - Name\t: %s\n", re.Provider.Infra.Name)
		fmt.Fprintf(tbWriter, " - ClusterID\t: %s\n", re.Provider.Version.OpenShift.ClusterID)
		fmt.Fprintf(tbWriter, " - Topology\t: %s\n", re.Provider.Infra.Topology)
		fmt.Fprintf(tbWriter, " - ControlPlaneTopology\t: %s\n", re.Provider.Infra.ControlPlaneTopology)
		fmt.Fprintf(tbWriter, " - API Server URL\t: %s\n", re.Provider.Infra.APIServerURL)
		fmt.Fprintf(tbWriter, " - API Server URL (internal)\t: %s\n", re.Provider.Infra.APIServerInternalURL)
		// fmt.Fprintf(tbWriter, " - Install Type\t: %s\n", "TODO (IPI or UPI?)")
		fmt.Fprintf(tbWriter, " - NetworkType\t: %s\n", re.Provider.Infra.NetworkType)
		// fmt.Fprintf(tbWriter, " - Proxy Configured\t: %s\n", "TODO (HTTP and/or HTTPS)")
	}

	fmt.Fprint(tbWriter, newLineWithTab)
	fmt.Fprintf(tbWriter, " Plugins summary by name:\t  Status [Total/Passed/Failed/Skipped] (timeout)\n")

	pluginName := plugin.PluginNameKubernetesConformance
	if _, ok := re.Provider.Plugins[pluginName]; !ok {
		errlog.LogError(errors.New(fmt.Sprintf("Unable to load plugin %s", pluginName)))
	}
	plK8S := re.Provider.Plugins[pluginName]
	name := plK8S.Name
	stat := plK8S.Stat
	pOCPPluginRes := fmt.Sprintf("%s [%d/%d/%d/%d] (%d)", stat.Status, stat.Total, stat.Passed, stat.Failed, stat.Skipped, stat.Timeout)
	if baselineProcessed {
		plK8S = re.Baseline.Plugins[pluginName]
		stat := plK8S.Stat
		bOCPPluginRes := fmt.Sprintf("%s [%d/%d/%d/%d] (%d)", stat.Status, stat.Total, stat.Passed, stat.Failed, stat.Skipped, stat.Timeout)
		fmt.Fprintf(tbWriter, " - %s\t: %s\t: %s\n", name, pOCPPluginRes, bOCPPluginRes)
	} else {
		fmt.Fprintf(tbWriter, " - %s\t: %s\n", name, pOCPPluginRes)
	}

	pluginName = plugin.PluginNameKubernetesConformance
	if _, ok := re.Provider.Plugins[pluginName]; !ok {
		errlog.LogError(errors.New(fmt.Sprintf("Unable to load plugin %s", pluginName)))
	}
	plOCP := re.Provider.Plugins[pluginName]
	name = plOCP.Name
	stat = plOCP.Stat
	pOCPPluginRes = fmt.Sprintf("%s [%d/%d/%d/%d] (%d)", stat.Status, stat.Total, stat.Passed, stat.Failed, stat.Skipped, stat.Timeout)
	if baselineProcessed {
		plOCP = re.Baseline.Plugins[pluginName]
		stat = plOCP.Stat
		bOCPPluginRes := fmt.Sprintf("%s [%d/%d/%d/%d] (%d)", stat.Status, stat.Total, stat.Passed, stat.Failed, stat.Skipped, stat.Timeout)
		fmt.Fprintf(tbWriter, " - %s\t: %s\t: %s\n", name, pOCPPluginRes, bOCPPluginRes)
	} else {
		fmt.Fprintf(tbWriter, " - %s\t: %s\n", name, pOCPPluginRes)
	}

	fmt.Fprint(tbWriter, newLineWithTab)
	fmt.Fprintf(tbWriter, " Health summary:\t  [A=True/P=True/D=True]\t\n")

	pOCPCO := re.Provider.ClusterOperators
	if baselineProcessed {
		bOCPCO := re.Baseline.ClusterOperators
		fmt.Fprintf(tbWriter, " - Cluster Operators\t: [%d/%d/%d]\t: [%d/%d/%d]\n",
			pOCPCO.CountAvailable, pOCPCO.CountProgressing, pOCPCO.CountDegraded,
			bOCPCO.CountAvailable, bOCPCO.CountProgressing, bOCPCO.CountDegraded,
		)
	} else {
		fmt.Fprintf(tbWriter, " - Cluster Operators\t: [%d/%d/%d]\n",
			pOCPCO.CountAvailable, pOCPCO.CountProgressing, pOCPCO.CountDegraded,
		)
	}

	// Show Nodes Health info collected by Sonobuoy
	pNhMessage := fmt.Sprintf("%d/%d %s", re.Provider.ClusterHealth.NodeHealthy, re.Provider.ClusterHealth.NodeHealthTotal, "")
	if re.Provider.ClusterHealth.NodeHealthTotal != 0 {
		pNhMessage = fmt.Sprintf("%s (%.2f%%)", pNhMessage, re.Provider.ClusterHealth.NodeHealthPerc)
	}

	if baselineProcessed {
		bNhMessage := fmt.Sprintf("%d/%d %s", re.Baseline.ClusterHealth.NodeHealthy, re.Baseline.ClusterHealth.NodeHealthTotal, "")
		if re.Baseline.ClusterHealth.NodeHealthTotal != 0 {
			bNhMessage = fmt.Sprintf("%s (%.2f%%)", bNhMessage, re.Baseline.ClusterHealth.NodeHealthPerc)
		}
		fmt.Fprintf(tbWriter, " - Node health\t: %s\t: %s\n", pNhMessage, bNhMessage)
	} else {
		fmt.Fprintf(tbWriter, " - Node health\t: %s\n", pNhMessage)
	}

	// Show Pods Health info collected by Sonobuoy
	pPodsHealthMsg := ""
	bPodsHealthMsg := ""
	phTotal := ""

	if re.Provider.ClusterHealth.PodHealthTotal != 0 {
		phTotal = fmt.Sprintf(" (%.2f%%)", re.Provider.ClusterHealth.PodHealthPerc)
	}
	pPodsHealthMsg = fmt.Sprintf("%d/%d %s", re.Provider.ClusterHealth.PodHealthy, re.Provider.ClusterHealth.PodHealthTotal, phTotal)

	if baselineProcessed {
		phTotal := ""
		if re.Baseline.ClusterHealth.PodHealthTotal != 0 {
			phTotal = fmt.Sprintf(" (%.2f%%)", re.Baseline.ClusterHealth.PodHealthPerc)
		}
		bPodsHealthMsg = fmt.Sprintf("%d/%d %s", re.Baseline.ClusterHealth.PodHealthy, re.Baseline.ClusterHealth.PodHealthTotal, phTotal)
		fmt.Fprintf(tbWriter, " - Pods health\t: %s\t: %s\n", pPodsHealthMsg, bPodsHealthMsg)
	} else {
		fmt.Fprintf(tbWriter, " - Pods health\t: %s\n", pPodsHealthMsg)
	}

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

func showProcessedSummary(re *report.Report) error {
	fmt.Printf("\n> Processed Summary <\n")

	fmt.Printf("\n Total tests by conformance suites:\n")
	checkEmpty := func(counter int) string {
		if counter == 0 {
			return "(FAIL)"
		}
		return ""
	}
	total := re.Provider.Plugins[plugin.PluginNameKubernetesConformance].Suite.Count
	fmt.Printf(" - %s: %d %s\n", summary.SuiteNameKubernetesConformance, total, checkEmpty(total))
	total = re.Provider.Plugins[plugin.PluginNameOpenShiftConformance].Suite.Count
	fmt.Printf(" - %s: %d %s\n", summary.SuiteNameOpenshiftConformance, total, checkEmpty(total))

	fmt.Printf("\n Result Summary by conformance plugins:\n")
	bProcessed := re.Provider.HasValidBaseline
	showSummaryPlugin(re.Provider, plugin.PluginNameKubernetesConformance, bProcessed)
	showSummaryPlugin(re.Provider, plugin.PluginNameOpenShiftConformance, bProcessed)
	showSummaryPlugin(re.Provider, plugin.PluginNameOpenShiftUpgrade, bProcessed)
	showSummaryPlugin(re.Provider, plugin.PluginNameArtifactsCollector, bProcessed)

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
	stat := p.Stat
	fmt.Printf(" - %s:\n", p.Name)
	fmt.Printf("   - Status: %s\n", stat.Status)
	fmt.Printf("   - Total: %d\n", stat.Total)
	fmt.Printf("   - Passed: %s\n", plugin.UtilsCalcPercStr(stat.Passed, stat.Total))
	fmt.Printf("   - Failed: %s\n", plugin.UtilsCalcPercStr(stat.Failed, stat.Total))
	fmt.Printf("   - Timeout: %s\n", plugin.UtilsCalcPercStr(stat.Timeout, stat.Total))
	fmt.Printf("   - Skipped: %s\n", plugin.UtilsCalcPercStr(stat.Skipped, stat.Total))
	if p.Name == plugin.PluginNameOpenShiftUpgrade || p.Name == plugin.PluginNameArtifactsCollector {
		return
	}
	// fmt.Printf("   - Failed (without filters) : %s\n", calcPercStr(int64(len(p.FailedList)), stat.Total))
	fmt.Printf("   - Failed (Filter SuiteOnly): %s\n", plugin.UtilsCalcPercStr(stat.FilterSuite, stat.Total))
	if bProcessed {
		fmt.Printf("   - Failed (Filter Baseline) : %s\n", plugin.UtilsCalcPercStr(stat.FilterBaseline, stat.Total))
	}
	fmt.Printf("   - Failed (Priority): %s\n", plugin.UtilsCalcPercStr(stat.FilterFailedPrio, stat.Total))

	// TODO: review suites provides better signal.
	// The final results for non-kubernetes conformance will be hidden (pass|fail) will be hiden for a while for those reasons:
	// - OPCT was created to provide feeaback of conformance results, not a passing binary value. The numbers should be interpreted
	// - Conformance results could have flakes or runtime failures which need to be investigated by executor
	// - Force user/executor to review the results, and not only the summary.
	// That behavior is aligned with BU: we expect kubernetes conformance passes in all providers, the reviewer
	// must set this as a target in the review process.
	if p.Name != plugin.PluginNameKubernetesConformance {
		return
	}
	// checking for runtime failures
	runtimeFailed := false
	if stat.Total == stat.Failed {
		runtimeFailed = true
	}

	// rewrite the original status when pass on all filters and not failed on runtime
	status := stat.Status
	if (stat.FilterFailedPrio == 0) && !runtimeFailed {
		status = "passed"
	}

	fmt.Printf("   - Status After Filters     : %s\n", status)
}

// showErrorDetails show details of failres for each plugin.
func showErrorDetails(re *report.Report, verbose bool) error {
	fmt.Printf("\n Result details by conformance plugins: \n")

	bProcessed := re.Provider.HasValidBaseline
	showErrorDetailPlugin(re.Provider.Plugins[plugin.PluginNameKubernetesConformance], verbose, bProcessed)
	showErrorDetailPlugin(re.Provider.Plugins[plugin.PluginNameOpenShiftConformance], verbose, bProcessed)

	return nil
}

// showErrorDetailPlugin Show failed e2e tests by filter, when verbose each filter will be shown.
func showErrorDetailPlugin(p *report.ReportPlugin, verbose bool, bProcessed bool) {
	flakeCount := p.Stat.FilterBaseline - p.Stat.FilterFailedPrio

	if verbose {
		fmt.Printf("\n\n => %s: (%d failures, %d failures filtered, %d flakes)\n", p.Name, p.Stat.Failed, p.Stat.FilterBaseline, flakeCount)

		fmt.Printf("\n --> [verbose] Failed tests detected on archive (without filters):\n")
		if p.Stat.Failed == 0 {
			fmt.Println("<empty>")
		}
		for _, test := range p.Tests {
			if test.State == "failed" {
				fmt.Println(test.Name)
			}
		}

		fmt.Printf("\n --> [verbose] Failed tests detected on suite (Filter SuiteOnly):\n")
		if p.Stat.FilterSuite == 0 {
			fmt.Println("<empty>")
		}
		for _, test := range p.Tests {
			if test.State == "filterSuiteOnly" {
				fmt.Println(test.Name)
			}
		}
		if bProcessed {
			fmt.Printf("\n --> [verbose] Failed tests removing baseline (Filter Baseline):\n")
			if p.Stat.FilterBaseline == 0 {
				fmt.Println("<empty>")
			}
			for _, test := range p.Tests {
				if test.State == "filterBaseline" {
					fmt.Println(test.Name)
				}
			}
		}
	} else {
		fmt.Printf("\n\n => %s: (%d failures, %d flakes)\n", p.Name, p.Stat.FilterBaseline, flakeCount)
	}

	fmt.Printf("\n --> Failed tests to Review (without flakes) - Immediate action:\n")
	noFlakes := make(map[string]struct{})
	if p.Stat.FilterBaseline == flakeCount {
		fmt.Println("<empty>")
	} else { // TODO move to small functions
		testTags := plugin.NewTestTagsEmpty(int(p.Stat.FilterFailedPrio))
		var testsWErrCnt []string
		for _, test := range p.TestsFailedPrio {
			noFlakes[test.Name] = struct{}{}
			testTags.Add(&test.Name)
			errCount := 0
			if _, ok := p.Tests[test.Name].ErrorCounters["total"]; ok {
				errCount = p.Tests[test.Name].ErrorCounters["total"]
			}
			testsWErrCnt = append(testsWErrCnt, fmt.Sprintf("%d\t%s", errCount, test.Name))
		}
		// Failed tests grouped by tag (first value between '[]')
		fmt.Printf("%s\n\n", testTags.ShowSorted())
		fmt.Println(strings.Join(testsWErrCnt[:], "\n"))
	}

	fmt.Printf("\n --> Failed flake tests - Statistic from OpenShift CI\n")
	tbWriter := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)

	if p.Stat.FilterBaseline == 0 {
		fmt.Fprintf(tbWriter, "<empty>\n")
	} else {
		testTags := plugin.NewTestTagsEmpty(int(p.Stat.FilterBaseline))
		fmt.Fprintf(tbWriter, "Flakes\tPerc\tErrCount\t TestName\n")
		for _, test := range p.TestsFlakeCI {
			// preventing duplication when flake tests was already listed.
			if _, ok := noFlakes[test.Name]; ok {
				continue
			}
			// TODO: fix issues when retrieving flakes from Sippy API.
			// Fallback to '--' when has issues.
			if p.Tests[test.Name].Flake == nil {
				fmt.Fprintf(tbWriter, "--\t--\t%s\n", test.Name)
			} else if p.Tests[test.Name].Flake.CurrentFlakes != 0 {
				errCount := 0
				if _, ok := p.Tests[test.Name].ErrorCounters["total"]; ok {
					errCount = p.Tests[test.Name].ErrorCounters["total"]
				}
				fmt.Fprintf(tbWriter, "%d\t%.3f%%\t%d\t%s\n",
					p.Tests[test.Name].Flake.CurrentFlakes,
					p.Tests[test.Name].Flake.CurrentFlakePerc,
					errCount, test.Name)
			}
			testTags.Add(&test.Name)
		}
		fmt.Printf("%s\n\n", testTags.ShowSorted())
	}
	tbWriter.Flush()
}

func showChecks(re *report.Report) error {

	tbWriter := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
	fmt.Fprintf(tbWriter, "\n> Presubmit Validation Checks\t\n")
	fmt.Fprintf(tbWriter, "\n>> Failed checks (must be reviewed before submitting the results):\t\n")
	for _, check := range re.Checks.Fail {
		name := check.Name
		if check.ID != "" {
			name = fmt.Sprintf("[%s] %s", check.ID, check.Name)
		}
		fmt.Fprintf(tbWriter, " - %s\t: %s\n", name, check.Result)
	}

	fmt.Fprintf(tbWriter, "\t\n>> Passed checks:\t\n")
	for _, check := range re.Checks.Pass {
		name := check.Name
		if check.ID != "" {
			name = fmt.Sprintf("[%s] %s", check.ID, check.Name)
		}
		fmt.Fprintf(tbWriter, " - %s\t: %s\n", name, check.Result)
	}

	fmt.Fprintf(tbWriter, "\n> Check the docs for each rule at %s\n", re.Checks.BaseURL)
	tbWriter.Flush()
	return nil
}
