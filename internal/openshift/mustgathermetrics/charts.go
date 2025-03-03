package mustgathermetrics

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/ptr"
)

type MetricValue struct {
	Timestap time.Time
	Value    string
}

type PrometheusResultMetric struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"`
}

type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string                   `json:"resultType"`
		Result     []PrometheusResultMetric `json:"result"`
	} `json:"data"`
}

type readMetricInput struct {
	filename string
	label    string
	title    string
	subtitle string
}

// newMetricsPage create the page object to genera the metric report.
func newMetricsPage() *components.Page {
	page := components.NewPage()
	page.PageTitle = "OPCT Report Metrics"
	return page
}

// SaveMetricsPageReport Create HTML metrics file in a given path.
func SaveMetricsPageReport(page *components.Page, path string) error {

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := page.Render(io.MultiWriter(f)); err != nil {
		return err
	}
	return nil
}

func (mmm *MustGatherChart) NewChart() *charts.Line {
	return mmm.processMetric(&readMetricInput{
		filename: mmm.Path,
		label:    mmm.PlotLabel,
		title:    mmm.PlotTitle,
		subtitle: mmm.PlotSubTitle,
	})
}

func (mmm *MustGatherChart) NewCharts() []*charts.Line {
	in := &readMetricInput{
		filename: mmm.Path,
		label:    mmm.PlotLabel,
		title:    mmm.PlotTitle,
		subtitle: mmm.PlotSubTitle,
	}
	return mmm.processMetrics(in)
}

// LoadData generates the metric widget (plot graph from data series).
func (mmm *MustGatherChart) LoadData(payload []byte) error {
	mmm.MetricData = &PrometheusResponse{}

	err := json.Unmarshal(payload, &mmm.MetricData)
	if err != nil {
		log.Errorf("Metrics/Extractor/Processing/LoadMetric ERROR parsing metric data: %v", err)
		return err
	}
	log.Debugf("Metrics/Extractor/Processing/LoadMetric Status: %s\n", mmm.MetricData.Status)
	return nil
}

// processMetric generates the metric widget (plot graph from data series).
func (mmm *MustGatherChart) processMetric(in *readMetricInput) *charts.Line {

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    in.title,
			Subtitle: in.subtitle,
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: ptr.To(true), Trigger: "axis"}),
	)

	allTimestamps := []string{}

	type ChartData struct {
		Label      string
		DataPoints []opts.LineData
	}

	chartData := []ChartData{}
	idx := 0
	for _, res := range mmm.MetricData.Data.Result {
		chart := ChartData{
			Label:      res.Metric[in.label],
			DataPoints: make([]opts.LineData, 0),
		}
		for _, datapoints := range res.Values {
			value := datapoints[1].(string)
			if value == "" {
				log.Debugf("Metrics/Extractor/Processing/GenChart: Empty value [%s], ignoring...", value)
				continue
			}
			// Convert from Unix timestamp to string value
			tm := time.Unix(int64(datapoints[0].(float64)), 0)
			strTimestamp := fmt.Sprintf("%d-%d-%d %d:%d:%d", tm.Year(), tm.Month(), tm.Day(), tm.Hour(), tm.Minute(), tm.Second())

			allTimestamps = append(allTimestamps, strTimestamp)
			chart.DataPoints = append(chart.DataPoints, opts.LineData{
				Value:      value,
				XAxisIndex: idx,
			})
			idx += 1
		}
		chartData = append(chartData, chart)
	}

	// sort.Strings(allTimestamps)
	line.SetXAxis(allTimestamps).
		SetSeriesOptions(charts.WithLineChartOpts(
			opts.LineChart{Smooth: ptr.To(false), ShowSymbol: ptr.To(true), SymbolSize: 15, Symbol: "diamond"},
		))
	for _, chart := range chartData {
		line.AddSeries(chart.Label, chart.DataPoints)
	}

	return line
}

// processMetric generates the metric widget (plot graph from data series).
func (mmm *MustGatherChart) processMetrics(in *readMetricInput) []*charts.Line {

	var lines []*charts.Line
	idx := 0
	for _, res := range mmm.MetricData.Data.Result {
		allTimestamps := []string{}
		line := charts.NewLine()
		line.SetGlobalOptions(
			charts.WithTitleOpts(opts.Title{
				Title:    in.title,
				Subtitle: in.subtitle,
			}),
			charts.WithTooltipOpts(opts.Tooltip{Show: ptr.To(true), Trigger: "axis"}),
		)
		dataPoints := make([]opts.LineData, 0)
		for _, datapoints := range res.Values {
			value := datapoints[1].(string)
			if value == "" {
				log.Debugf("Metrics/Extractor/Processing/GenChart: Empty value [%s], ignoring...", value)
				continue
			}
			// Convert from Unix timestamp to string value
			tm := time.Unix(int64(datapoints[0].(float64)), 0)
			strTimestamp := fmt.Sprintf("%d-%d-%d %d:%d:%d", tm.Year(), tm.Month(), tm.Day(), tm.Hour(), tm.Minute(), tm.Second())

			allTimestamps = append(allTimestamps, strTimestamp)
			dataPoints = append(dataPoints, opts.LineData{
				Value:      value,
				XAxisIndex: idx,
			})
			idx += 1
		}
		line.SetXAxis(allTimestamps).
			SetSeriesOptions(charts.WithLineChartOpts(
				opts.LineChart{Smooth: ptr.To(false), ShowSymbol: ptr.To(true), SymbolSize: 15, Symbol: "diamond"},
			))
		line.AddSeries(res.Metric[in.label], dataPoints)
		lines = append(lines, line)
	}

	// sort.Strings(allTimestamps)
	// line.SetSeriesOptions(charts.WithLineChartOpts(
	// 	opts.LineChart{Smooth: false, ShowSymbol: true, SymbolSize: 15, Symbol: "diamond"},
	// ))
	return lines
}
