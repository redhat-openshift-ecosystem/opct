package mustgathermetrics

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/chart"
	log "github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
)

type MustGatherMetrics struct {
	fileName        string
	data            *bytes.Buffer
	ReportPath      string
	ReportChartFile string
	ServePath       string
}

func NewMustGatherMetrics(report, file, uri string, data *bytes.Buffer) (*MustGatherMetrics, error) {
	return &MustGatherMetrics{
		fileName:        filepath.Base(file),
		data:            data,
		ReportPath:      report,
		ReportChartFile: "/metrics.html",
		ServePath:       uri,
	}, nil
}

func (mg *MustGatherMetrics) Process() error {
	log.Debugf("Processing results/Populating/Populating Summary/Processing/MustGather/Reading")
	tar, err := mg.read(mg.data)
	if err != nil {
		return err
	}
	log.Debugf("Processing results/Populating/Populating Summary/Processing/MustGather/Processing")
	err = mg.extract(tar)
	if err != nil {
		return err
	}
	return nil
}

func (mg *MustGatherMetrics) read(buf *bytes.Buffer) (*tar.Reader, error) {
	file, err := xz.NewReader(buf)
	if err != nil {
		return nil, err
	}
	return tar.NewReader(file), nil
}

// extract dispatch to process must-gather items.
func (mg *MustGatherMetrics) extract(tarball *tar.Reader) error {

	keepReading := true
	metricsPage := chart.NewMetricsPage()
	reportPath := mg.ReportPath + mg.ReportChartFile
	page := chart.NewMetricsPageWithPlotly(mg.ReportPath, mg.ServePath)

	// Walk through files in tarball file.
	for keepReading {
		header, err := tarball.Next()

		switch {

		// no more files
		case err == io.EOF:

			err := chart.SaveMetricsPageReport(metricsPage, reportPath)
			if err != nil {
				log.Errorf("error saving metrics to: %s\n", reportPath)
				return err
			}
			// Ploty Page
			log.Debugf("Generating Charts with Plotly\n")
			err = page.RenderPage()
			if err != nil {
				log.Errorf("error rendering page: %v\n", err)
				return err
			}

			log.Debugf("metrics saved at: %s\n", reportPath)
			return nil

		// return on error
		case err != nil:
			return errors.Wrapf(err, "error reading tarball")

		// skip it when the headr isn't set (not sure how this happens)
		case header == nil:
			continue
		}

		// process only metris file. Example: monitoring/prometheus/metrics/metric.json.gz
		if !(strings.HasPrefix(header.Name, "monitoring/prometheus/metrics") && strings.HasSuffix(header.Name, ".json.gz")) {
			continue
		}

		metricFileName := filepath.Base(header.Name)

		chart, ok := chart.ChartsAvailable[metricFileName]
		if !ok {
			log.Debugf("Metrics/Extractor/Unsupported metric, ignoring metric data %s\n", header.Name)
			continue
		}
		if !chart.CollectorAvailable {
			log.Debugf("Metrics/Extractor/No charts available for metric %s\n", header.Name)
			continue
		}
		log.Debugf("Metrics/Extractor/Processing: %s\n", header.Name)

		gz, err := gzip.NewReader(tarball)
		if err != nil {
			log.Debugf("Metrics/Extractor/Processing/ERROR reading metric %v", err)
			continue
		}
		defer gz.Close()
		var metricPayload bytes.Buffer
		if _, err := io.Copy(&metricPayload, gz); err != nil {
			log.Debugf("Metrics/Extractor/Processing/ERROR copying metric data for %v", err)
			continue
		}

		err = chart.LoadData(metricPayload.Bytes())
		if err != nil {
			log.Debugf("Metrics/Extractor/Processing/ERROR loading metric for %v", err)
			continue
		}

		// charts with
		for _, line := range chart.NewCharts() {
			metricsPage.AddCharts(line)
		}
		log.Debugf("Metrics/Extractor/Processing/Done %v", header.Name)
	}

	return nil
}
