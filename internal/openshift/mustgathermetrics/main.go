package mustgathermetrics

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
)

type MustGatherChart struct {
	Path               string
	OriginalQuery      string
	PlotLabel          string
	PlotTitle          string
	PlotSubTitle       string
	CollectorAvailable bool
	MetricData         *PrometheusResponse
	DivId              string
}

type MustGatherCharts map[string]*MustGatherChart

type MustGatherMetrics struct {
	fileName        string
	data            *bytes.Buffer
	ReportPath      string
	ReportChartFile string
	ServePath       string
	charts          MustGatherCharts
	page            *ChartPagePlotly
}

func NewMustGatherMetrics(report, file, uri string, data *bytes.Buffer) (*MustGatherMetrics, error) {
	mgm := &MustGatherMetrics{
		fileName:        filepath.Base(file),
		data:            data,
		ReportPath:      report,
		ServePath:       uri,
		ReportChartFile: "/metrics.html",
	}

	mgm.charts = make(map[string]*MustGatherChart, 0)
	mgm.charts["query_range-etcd-disk-fsync-db-duration-p99.json.gz"] = &MustGatherChart{
		Path:               "query_range-etcd-disk-fsync-db-duration-p99.json.gz",
		OriginalQuery:      "",
		PlotLabel:          "instance",
		PlotTitle:          "etcd fsync DB p99",
		PlotSubTitle:       "",
		CollectorAvailable: true,
		DivId:              "id1",
	}
	mgm.charts["query_range-api-kas-request-duration-p99.json.gz"] = &MustGatherChart{
		Path:               "query_range-api-kas-request-duration-p99.json.gz",
		OriginalQuery:      "",
		PlotLabel:          "verb",
		PlotTitle:          "Kube API request p99",
		PlotSubTitle:       "",
		CollectorAvailable: true,
		DivId:              "id2",
	}
	mgm.charts["query_range-etcd-disk-fsync-wal-duration-p99.json.gz"] = &MustGatherChart{
		Path:               "query_range-etcd-disk-fsync-wal-duration-p99.json.gz",
		OriginalQuery:      "",
		PlotLabel:          "instance",
		PlotTitle:          "etcd fsync WAL p99",
		PlotSubTitle:       "",
		CollectorAvailable: true,
		DivId:              "id0",
	}
	mgm.charts["query_range-etcd-peer-round-trip-time.json.gz"] = &MustGatherChart{
		Path:               "query_range-etcd-peer-round-trip-time.json.gz",
		OriginalQuery:      "",
		PlotLabel:          "instance",
		PlotTitle:          "etcd peer round trip",
		PlotSubTitle:       "",
		CollectorAvailable: true,
		DivId:              "id3",
	}

	mgm.charts["query_range-etcd-total-leader-elections-day.json.gz"] = &MustGatherChart{
		Path:               "query_range-etcd-total-leader-elections-day.json.gz",
		OriginalQuery:      "",
		PlotLabel:          "instance",
		PlotTitle:          "etcd peer total leader election",
		PlotSubTitle:       "",
		CollectorAvailable: true,
		DivId:              "id4",
	}
	mgm.charts["query_range-etcd-request-duration-p99.json.gz"] = &MustGatherChart{
		Path:               "query_range-etcd-request-duration-p99.json.gz",
		OriginalQuery:      "",
		PlotLabel:          "operation",
		PlotTitle:          "etcd req duration p99",
		PlotSubTitle:       "",
		CollectorAvailable: true,
		DivId:              "id5",
	}
	mgm.charts["query_range-cluster-storage-iops.json.gz"] = &MustGatherChart{
		Path:               "query_range-cluster-storage-iops.json.gz",
		OriginalQuery:      "",
		PlotLabel:          "namespace",
		PlotTitle:          "Cluster storage IOPS",
		PlotSubTitle:       "",
		CollectorAvailable: false,
		DivId:              "id6",
	}
	mgm.charts["query_range-cluster-storage-throughput.json.gz"] = &MustGatherChart{
		Path:               "query_range-cluster-storage-throughput.json.gz",
		OriginalQuery:      "",
		PlotLabel:          "namespace",
		PlotTitle:          "Cluster storage throughput",
		PlotSubTitle:       "",
		CollectorAvailable: false,
		DivId:              "id7",
	}
	mgm.charts["query_range-cluster-cpu-usage.json.gz"] = &MustGatherChart{
		Path:               "query_range-cluster-cpu-usage.json.gz",
		OriginalQuery:      "",
		PlotLabel:          "namespace",
		PlotTitle:          "Cluster CPU",
		PlotSubTitle:       "",
		CollectorAvailable: false,
		DivId:              "id8",
	}
	mgm.page = newMetricsPageWithPlotly(report, uri, mgm.charts)
	return mgm, nil
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
	metricsPage := newMetricsPage()
	reportPath := mg.ReportPath + mg.ReportChartFile

	// Walk through files in tarball.
	for keepReading {
		header, err := tarball.Next()

		switch {

		// no more files
		case err == io.EOF:

			err := SaveMetricsPageReport(metricsPage, reportPath)
			if err != nil {
				log.Errorf("error saving metrics to: %s\n", reportPath)
				return err
			}
			// Ploty Page
			log.Debugf("Generating Charts with Plotly\n")
			err = mg.page.RenderPage()
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

		chart, ok := mg.charts[metricFileName]
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
