package adm

import (
	"bufio"
	"bytes"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/redhat-openshift-ecosystem/opct/internal/openshift/mustgathermetrics"
)

type parseMetricsInput struct {
	input  string
	output string
}

var parseMetricsArgs parseMetricsInput
var parseMetricsCmd = &cobra.Command{
	Use:     "parse-metrics",
	Example: "opct adm parse-metrics --input ./metrics.tar.xz --output /tmp/metrics",
	Short:   "Process the metrics collected by OPCT and create a HTML report graph.",
	Run:     parseMetricsRun,
}

func init() {
	parseMetricsCmd.Flags().StringVar(&parseMetricsArgs.input, "input", "", "Input metrics file. Example: metrics.tar.xz")
	parseMetricsCmd.Flags().StringVar(&parseMetricsArgs.output, "output", "", "Output directory. Example: /tmp/metrics")
}

func parseMetricsRun(cmd *cobra.Command, args []string) {

	if parseMetricsArgs.input == "" {
		log.Error("missing argumet --input <metric archive file.tar.xz>")
		os.Exit(1)
	}

	if parseMetricsArgs.output == "" {
		log.Error("missing argumet --output <target directory to save parsed metrics>")
		os.Exit(1)
	}

	log.Infof("Start metrics parser for metric data %s", parseMetricsArgs.input)

	fi, err := os.Open(parseMetricsArgs.input)
	if err != nil {
		panic(err)
	}
	// close fi on exit and check for its returned error
	defer func() {
		if err := fi.Close(); err != nil {
			panic(err)
		}
	}()

	r := bufio.NewReader(fi)
	buf := &bytes.Buffer{}
	_, err = buf.ReadFrom(r)
	if err != nil {
		log.Errorf("unable to read buffer: %v", err)
		panic(err)
	}

	htmlFile := "/metrics.html"
	mgm, err := mustgathermetrics.NewMustGatherMetrics(parseMetricsArgs.output, htmlFile, "/", buf)
	if err != nil {
		log.Errorf("unable to read metric archive: %v", err)
		panic(err)
	}
	err = mgm.Process()
	if err != nil {
		log.Errorf("processing metric: %v", err)
		os.Exit(1)
	}
	log.Infof("Success! HTML report created at %s/%s\n", parseMetricsArgs.output, htmlFile)
	log.Infof("TIP: cd %s && python -m http.server", parseMetricsArgs.output)
	log.Info("Open your browser and navigate the reports: http://localhost:8000/index.html http://localhost:8000/metrics.html")
}
