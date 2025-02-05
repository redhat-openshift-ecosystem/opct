package mustgathermetrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"text/template"
	"time"

	log "github.com/sirupsen/logrus"
)

type ChartPagePlotly struct {
	PageTitle string
	Charts    MustGatherCharts
	RootPath  string
	UriPath   string
}

const indexHTML = `<!DOCTYPE html>
<html>
	<head>
		<title>OPCT Charts</title>
		<script src="https://cdn.plot.ly/plotly-2.8.3.min.js"></script>
		<script src="./index.js"></script>
		<style>
				#chart {
					width: 100px;
					height: 100px;
				}
		</style>
	</head>
	<body onload="updateCharts()">
		<hr />
{{.Table}}
	</body>
</html>`

// inspired by https://github.com/353words/stocks/tree/main
const indexJS = `
async function updateCharts() {
    let chartsResp = await fetch('./index.json');
    let charts = await chartsResp.json(); 
    for (idx in charts) {
        let resp = await fetch(charts[idx].path);
        let reply = await resp.json(); 
        Plotly.newPlot(charts[idx].id, reply.data, reply.layout);
    }
}`

func newMetricsPageWithPlotly(path, uri string, charts MustGatherCharts) *ChartPagePlotly {

	page := &ChartPagePlotly{
		PageTitle: "OPCT Report Metrics",
		Charts:    charts,
		RootPath:  path,
		UriPath:   uri,
	}

	// create base dir
	err := os.Mkdir(page.RootPath, 0755)
	if err != nil {
		log.Errorf("Unable to create directory %s: %v", page.RootPath, err)
	}
	log.Debugf("Chart/Directory created %s", page.RootPath)
	return page
}

func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func (cpp *ChartPagePlotly) RenderPage() error {

	// - index.js
	indexJsFilePath := fmt.Sprintf("%s/index.js", cpp.RootPath)
	err := os.WriteFile(indexJsFilePath, []byte(indexJS), 0644)
	if err != nil {
		log.Errorf("Unable to save file %s: %v", indexJsFilePath, err)
	}
	log.Debugf("Chart/file saved %s", indexJsFilePath)

	// render metrics data
	indexChartsMap := []map[string]string{}
	validDivIds := []string{}
	for k := range cpp.Charts {
		if err := cpp.processMetricV2(k); err != nil {
			log.Debug(err)
			continue
		}
		if cpp.Charts[k].DivId != "" {
			indexChartsMap = append(indexChartsMap, map[string]string{
				"id":   cpp.Charts[k].DivId,
				"path": fmt.Sprintf("./%s.json", cpp.Charts[k].Path),
			})
			validDivIds = append(validDivIds, cpp.Charts[k].DivId)
		}
	}
	// create table with charts
	sort.Strings(validDivIds)
	type TemplateData struct {
		Table string
	}
	table := TemplateData{"\t\t<table>"}
	for idx, div := range validDivIds {
		if idx%2 == 0 {
			table.Table += fmt.Sprintf("\n\t\t\t<tr><td><div id=\"%s\"></div></td>", div)
		} else {
			table.Table += fmt.Sprintf("<td><div id=\"%s\"></div></td></tr>", div)
		}
	}
	table.Table += "\n\t\t</table>"

	// - index.html
	indexHTMLFilePath := fmt.Sprintf("%s/index.html", cpp.RootPath)
	tmplS, err := template.New("report").Parse(indexHTML)
	if err != nil {
		log.Errorf("Unable to create template for %s: %v", indexHTMLFilePath, err)
	}
	var fileBufferS bytes.Buffer
	err = tmplS.Execute(&fileBufferS, table)
	if err != nil {
		log.Errorf("Unable to render template for %s: %v", indexHTMLFilePath, err)
	}

	err = os.WriteFile(indexHTMLFilePath, fileBufferS.Bytes(), 0644)
	if err != nil {
		log.Errorf("Unable to save file %s: %v", indexHTMLFilePath, err)
	}
	log.Debugf("Chart/file saved %s", indexHTMLFilePath)

	// - index.json
	indexJsonFileData, _ := json.MarshalIndent(indexChartsMap, "", " ")
	indexJsonFilePath := fmt.Sprintf("%s/index.json", cpp.RootPath)
	err = os.WriteFile(indexJsonFilePath, indexJsonFileData, 0644)
	if err != nil {
		log.Errorf("Unable to save file %s: %v", indexJsonFileData, err)
	}
	log.Debugf("Chart/file saved %s", indexJsonFilePath)
	return nil
}

// processMetric generates the metric widget (plot graph from data series).
func (cpp *ChartPagePlotly) processMetricV2(name string) error {

	chart := cpp.Charts[name]
	type LabelData struct {
		Name  string
		XAxis []string
		YAxis []float64
	}

	type Labels map[string]LabelData

	// process query
	labels := make(Labels, 0)
	if chart.MetricData == nil {
		return fmt.Errorf("empty metric data, ignoring metric %s", name)
	}
	log.Debugf("Processing metric %s", name)
	for _, res := range chart.MetricData.Data.Result {
		// process labels
		for _, datapoints := range res.Values {
			value := datapoints[1].(string)
			if value == "" {
				log.Debugf("Metrics/Extractor/Processing/GenChart: Empty value [%s], ignoring...", value)
				continue
			}
			// Convert from Unix timestamp to string value
			tm := time.Unix(int64(datapoints[0].(float64)), 0)
			strTimestamp := fmt.Sprintf("%d-%d-%d %d:%d:%d", tm.Year(), tm.Month(), tm.Day(), tm.Hour(), tm.Minute(), tm.Second())

			valF, err := strconv.ParseFloat(value, 64)
			if err != nil {
				log.Errorf("error metric %s: converting datapoint, ignoring", name)
				continue
			}

			labelValue := res.Metric[chart.PlotLabel]
			if _, ok := labels[labelValue]; !ok {
				labels[labelValue] = LabelData{Name: labelValue}
			}

			label := labels[labelValue]
			label.XAxis = append(label.XAxis, strTimestamp)
			label.YAxis = append(label.YAxis, roundFloat(valF, 4))
			labels[labelValue] = label
		}
	}

	var data []map[string]interface{}
	count := 1
	for _, label := range labels {
		dataAxis := map[string]interface{}{
			"x":           label.XAxis,
			"y":           label.YAxis,
			"name":        label.Name,
			"type":        "scatter",
			"connectgaps": true,
			"mode":        "lines+markers",
		}
		if count != 1 {
			dataAxis["yaxis"] = fmt.Sprintf("y%d", count)
		}
		data = append(data, dataAxis)
		count += 1
	}

	if len(data) == 0 {
		return fmt.Errorf("error processing metric: no valid data for %q", name)
	}

	// create table with rows by label
	reply := map[string]interface{}{
		"data": data,
		"layout": map[string]interface{}{
			"title": chart.PlotTitle,
			"grid": map[string]int{
				"rows":    len(labels),
				"columns": 1,
			},
			"autosize": false,
			"width":    1000,
			"height":   1000,
		},
	}

	indexJsonFileData, err := json.MarshalIndent(reply, "", " ")
	if err != nil {
		// log.Errorf("Unable to unmarshall metric file %v", err)
		return fmt.Errorf("unable to unmarshall metric file %v", err)
	}

	indexJsonFilePath := fmt.Sprintf("%s/%s.json", cpp.RootPath, chart.Path)
	err = os.WriteFile(indexJsonFilePath, indexJsonFileData, 0644)
	if err != nil {
		// log.Errorf("Unable to save file %s: %v", indexJsonFilePath, err)
		return fmt.Errorf("unable to save file %s: %v", indexJsonFilePath, err)
	}
	return nil
}
