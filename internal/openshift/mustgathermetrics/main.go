package mustgathermetrics

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
)

//go:embed charts-config.json
var chartsConfigJSON []byte

//go:embed metrics.html
var metricsHTML []byte

//go:embed index.html
var indexHTML []byte

// ChartConfig represents a single metric chart configuration
type ChartConfig struct {
	File  string `json:"file"`
	Label string `json:"label"`
	Title string `json:"title"`
	ID    string `json:"id"`
}

// ChartsConfig represents the configuration file structure
type ChartsConfig struct {
	Charts []ChartConfig `json:"charts"`
}

// PrometheusResultMetric represents a single result from Prometheus query_range API
type PrometheusResultMetric struct {
	Metric map[string]string `json:"metric"`
	Values [][]interface{}   `json:"values"`
}

// PrometheusResponse represents the response from Prometheus query_range API
type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string                   `json:"resultType"`
		Result     []PrometheusResultMetric `json:"result"`
	} `json:"data"`
}

// Chart represents a single metric chart with its data
type Chart struct {
	Config ChartConfig
	Data   *PrometheusResponse
}

// MustGatherMetrics processes metrics from must-gather archive
type MustGatherMetrics struct {
	reportPath string
	data       *bytes.Buffer
	charts     map[string]*Chart
}

// NewMustGatherMetrics creates a new metrics processor
func NewMustGatherMetrics(reportPath string, data *bytes.Buffer) (*MustGatherMetrics, error) {
	// Load chart configurations
	var config ChartsConfig
	if err := json.Unmarshal(chartsConfigJSON, &config); err != nil {
		return nil, fmt.Errorf("failed to load chart config: %w", err)
	}

	// Initialize charts map
	charts := make(map[string]*Chart)
	for _, chartCfg := range config.Charts {
		charts[chartCfg.File] = &Chart{
			Config: chartCfg,
		}
	}

	return &MustGatherMetrics{
		reportPath: reportPath,
		data:       data,
		charts:     charts,
	}, nil
}

// Process extracts and processes metrics from the must-gather archive
func (mg *MustGatherMetrics) Process() error {
	log.Debugf("Processing must-gather metrics archive")

	// Read tar.xz archive
	tarReader, err := mg.readArchive(mg.data)
	if err != nil {
		return fmt.Errorf("failed to read archive: %w", err)
	}

	// Extract metrics
	if err := mg.extractMetrics(tarReader); err != nil {
		return fmt.Errorf("failed to extract metrics: %w", err)
	}

	// Generate output files
	if err := mg.generateOutputFiles(); err != nil {
		return fmt.Errorf("failed to generate output files: %w", err)
	}

	log.Debugf("Metrics processing complete: %s", mg.reportPath)
	return nil
}

// readArchive reads the tar.xz archive
func (mg *MustGatherMetrics) readArchive(buf *bytes.Buffer) (*tar.Reader, error) {
	xzReader, err := xz.NewReader(buf)
	if err != nil {
		return nil, err
	}
	return tar.NewReader(xzReader), nil
}

// extractMetrics walks through the tar archive and extracts metric files
func (mg *MustGatherMetrics) extractMetrics(tarReader *tar.Reader) error {
	for {
		header, err := tarReader.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return fmt.Errorf("error reading tar: %w", err)
		case header == nil:
			continue
		}

		// Only process Prometheus metric files: monitoring/prometheus/metrics/*.json.gz
		if !strings.HasPrefix(header.Name, "monitoring/prometheus/metrics") {
			continue
		}
		if !strings.HasSuffix(header.Name, ".json.gz") {
			continue
		}

		fileName := filepath.Base(header.Name)
		chart, ok := mg.charts[fileName]
		if !ok {
			log.Debugf("Skipping unsupported metric: %s", fileName)
			continue
		}

		log.Debugf("Processing metric: %s", fileName)

		// Decompress gzip
		gzReader, err := gzip.NewReader(tarReader)
		if err != nil {
			log.Warnf("Failed to decompress %s: %v", fileName, err)
			continue
		}

		// Read metric data
		var metricPayload bytes.Buffer
		if _, err := io.Copy(&metricPayload, gzReader); err != nil {
			gzReader.Close()
			log.Warnf("Failed to read %s: %v", fileName, err)
			continue
		}
		gzReader.Close()

		// Parse Prometheus JSON
		var promResponse PrometheusResponse
		if err := json.Unmarshal(metricPayload.Bytes(), &promResponse); err != nil {
			log.Warnf("Failed to parse JSON for %s: %v", fileName, err)
			continue
		}

		chart.Data = &promResponse
		log.Debugf("Loaded metric: %s (status=%s)", fileName, promResponse.Status)
	}
}

// generateOutputFiles creates index.json and individual chart JSON files
func (mg *MustGatherMetrics) generateOutputFiles() error {
	// Create output directory
	if err := os.MkdirAll(mg.reportPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", mg.reportPath, err)
	}

	// Build index
	type IndexEntry struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	var index []IndexEntry

	// Process each chart
	for fileName, chart := range mg.charts {
		if chart.Data == nil {
			log.Debugf("Skipping chart %s: no data loaded", fileName)
			continue
		}

		// Filter NaN/Inf values (JSON doesn't support them)
		filteredData := mg.filterInvalidValues(chart.Data)

		// Check if any valid data remains
		hasValidData := false
		for _, result := range filteredData.Data.Result {
			if len(result.Values) > 0 {
				hasValidData = true
				break
			}
		}

		if !hasValidData {
			log.Warnf("Skipping chart %s: no valid data after filtering", fileName)
			continue
		}

		// Save chart JSON (Prometheus format)
		chartPath := filepath.Join(mg.reportPath, fileName+".json")
		if err := mg.saveChartJSON(chartPath, filteredData); err != nil {
			return fmt.Errorf("failed to save chart %s: %w", fileName, err)
		}

		// Add to index
		index = append(index, IndexEntry{
			ID:   chart.Config.ID,
			Path: fmt.Sprintf("./%s.json", fileName),
		})

		log.Debugf("Saved chart: %s", chartPath)
	}

	// Save index.json
	indexPath := filepath.Join(mg.reportPath, "index.json")
	if len(index) == 0 {
		return fmt.Errorf("no chart JSON files were generated")
	}

	indexJSON, err := json.MarshalIndent(index, "", " ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	if err := os.WriteFile(indexPath, indexJSON, 0644); err != nil {
		return fmt.Errorf("failed to write index.json: %w", err)
	}

	log.Debugf("Saved index: %s (%d charts)", indexPath, len(index))

	// Save metrics.html (interactive dashboard)
	metricsHTMLPath := filepath.Join(mg.reportPath, "metrics.html")
	if err := os.WriteFile(metricsHTMLPath, metricsHTML, 0644); err != nil {
		return fmt.Errorf("failed to write metrics.html: %w", err)
	}
	log.Debugf("Saved metrics dashboard: %s", metricsHTMLPath)

	// Save index.html (redirect to metrics.html)
	indexHTMLPath := filepath.Join(mg.reportPath, "index.html")
	if err := os.WriteFile(indexHTMLPath, indexHTML, 0644); err != nil {
		return fmt.Errorf("failed to write index.html: %w", err)
	}
	log.Debugf("Saved index redirect: %s", indexHTMLPath)

	return nil
}

// filterInvalidValues removes NaN and Inf values from Prometheus response
// JSON doesn't support these values, so they must be filtered out
func (mg *MustGatherMetrics) filterInvalidValues(data *PrometheusResponse) *PrometheusResponse {
	filtered := &PrometheusResponse{
		Status: data.Status,
	}
	filtered.Data.ResultType = data.Data.ResultType

	totalDatapoints := 0
	filteredDatapoints := 0

	for _, result := range data.Data.Result {
		validValues := [][]interface{}{}
		originalCount := len(result.Values)
		totalDatapoints += originalCount

		for _, v := range result.Values {
			// v[0] = timestamp (float64), v[1] = value (string)
			if len(v) < 2 {
				continue
			}

			valueStr, ok := v[1].(string)
			if !ok {
				continue
			}

			// Parse and check for NaN/Inf
			valueFloat, err := strconv.ParseFloat(valueStr, 64)
			if err != nil {
				continue
			}

			if math.IsNaN(valueFloat) || math.IsInf(valueFloat, 0) {
				filteredDatapoints++
				continue
			}

			// Valid value, keep it
			validValues = append(validValues, v)
		}

		// Only include results with valid data
		if len(validValues) > 0 {
			filteredResult := PrometheusResultMetric{
				Metric: result.Metric,
				Values: validValues,
			}
			filtered.Data.Result = append(filtered.Data.Result, filteredResult)
		}
	}

	// Log summary if data was filtered
	if filteredDatapoints > 0 {
		log.Debugf("Filtered %d NaN/Inf datapoints from %d total (%.1f%% invalid)",
			filteredDatapoints, totalDatapoints, float64(filteredDatapoints)/float64(totalDatapoints)*100)
	}

	return filtered
}

// saveChartJSON saves a chart's Prometheus data as JSON
func (mg *MustGatherMetrics) saveChartJSON(path string, data *PrometheusResponse) error {
	jsonData, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
