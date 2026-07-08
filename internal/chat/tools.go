package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/invopop/jsonschema"
)

func GenerateSchema[T any]() anthropic.ToolInputSchemaParam {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return anthropic.ToolInputSchemaParam{
		Properties: schema.Properties,
	}
}

type PluginNameInput struct {
	PluginName string `json:"plugin_name" jsonschema:"enum=10-openshift-kube-conformance,enum=20-openshift-conformance-validated,enum=05-openshift-cluster-upgrade,enum=80-openshift-tests-replay,enum=99-openshift-artifacts-collector" jsonschema_description:"The plugin name to query results for"`
}

type TestIDInput struct {
	TestID string `json:"test_id" jsonschema_description:"The test ID (e.g. 20-openshift-conformance-validated-3432)"`
}

func ToolDefinitions() []anthropic.ToolUnionParam {
	tools := []anthropic.ToolParam{
		{
			Name:        "read_report_summary",
			Description: anthropic.String("Get the OPCT report summary including cluster info, test statistics, alerts, and validation check overview. Always call this first to understand the report context."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "read_report_checks",
			Description: anthropic.String("Get all OPCT validation check results (failures, warnings, passes, skips) with SLO targets and current values"),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "read_plugin_results",
			Description: anthropic.String("Get test results for a specific plugin including statistics and filtered failure list"),
			InputSchema: GenerateSchema[PluginNameInput](),
		},
		{
			Name:        "list_failed_tests",
			Description: anthropic.String("List all failed tests for a plugin with their IDs, names, error counts, and filter states. Use this to get an overview before investigating individual failures."),
			InputSchema: GenerateSchema[PluginNameInput](),
		},
		{
			Name:        "read_test_failure_log",
			Description: anthropic.String("Read the failure log for a specific test by its ID. Use this to investigate root causes of test failures."),
			InputSchema: GenerateSchema[TestIDInput](),
		},
		{
			Name:        "read_test_output",
			Description: anthropic.String("Read the full test output (systemOut) for a specific test by its ID. Use this when the failure log alone is not enough to determine root cause."),
			InputSchema: GenerateSchema[TestIDInput](),
		},
		{
			Name:        "read_etcd_data",
			Description: anthropic.String("Get etcd performance data including error counters, slow request statistics, and per-pod breakdown from must-gather logs"),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]interface{}{},
			},
		},
		{
			Name:        "read_network_data",
			Description: anthropic.String("Get network connectivity check results including outages, failures, and check summaries"),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]interface{}{},
			},
		},
	}

	result := make([]anthropic.ToolUnionParam, len(tools))
	for i, t := range tools {
		t := t
		result[i] = anthropic.ToolUnionParam{OfTool: &t}
	}
	return result
}

type ToolExecutor struct {
	reportDir  string
	reportData map[string]interface{}
}

func NewToolExecutor(reportDir string) *ToolExecutor {
	return &ToolExecutor{
		reportDir: reportDir,
	}
}

func (te *ToolExecutor) loadReportData() (map[string]interface{}, error) {
	if te.reportData != nil {
		return te.reportData, nil
	}
	data, err := os.ReadFile(filepath.Join(te.reportDir, "opct-report.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read report: %w", err)
	}
	var report map[string]interface{}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse report: %w", err)
	}
	te.reportData = report
	return report, nil
}

func (te *ToolExecutor) Execute(toolName string, input json.RawMessage) (string, error) {
	switch toolName {
	case "read_report_summary":
		return te.readReportSummary()
	case "read_report_checks":
		return te.readReportChecks()
	case "read_plugin_results":
		return te.readPluginResults(input)
	case "list_failed_tests":
		return te.listFailedTests(input)
	case "read_test_failure_log":
		return te.readTestFailureLog(input)
	case "read_test_output":
		return te.readTestOutput(input)
	case "read_etcd_data":
		return te.readEtcdData()
	case "read_network_data":
		return te.readNetworkData()
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (te *ToolExecutor) readReportSummary() (string, error) {
	report, err := te.loadReportData()
	if err != nil {
		return "", err
	}
	result := map[string]interface{}{}
	if v, ok := report["summary"]; ok {
		result["summary"] = v
	}
	if provider, ok := report["provider"].(map[string]interface{}); ok {
		result["cluster_version"] = provider["version"]
		result["infrastructure"] = provider["infra"]
		result["cluster_operators"] = provider["clusterOperators"]
		result["cluster_health"] = provider["clusterHealth"]
		result["nodes"] = provider["nodes"]
	}
	if v, ok := report["setup"]; ok {
		result["setup"] = v
	}
	return marshalResult(result)
}

func (te *ToolExecutor) readReportChecks() (string, error) {
	report, err := te.loadReportData()
	if err != nil {
		return "", err
	}
	if v, ok := report["checks"]; ok {
		return marshalResult(v)
	}
	return "{}", nil
}

func (te *ToolExecutor) readPluginResults(input json.RawMessage) (string, error) {
	var params PluginNameInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	report, err := te.loadReportData()
	if err != nil {
		return "", err
	}
	provider, ok := report["provider"].(map[string]interface{})
	if !ok {
		return "{}", nil
	}
	plugins, ok := provider["plugins"].(map[string]interface{})
	if !ok {
		return "{}", nil
	}
	plugin, ok := plugins[params.PluginName]
	if !ok {
		return "", fmt.Errorf("plugin %q not found", params.PluginName)
	}
	return marshalResult(plugin)
}

func (te *ToolExecutor) listFailedTests(input json.RawMessage) (string, error) {
	var params PluginNameInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	report, err := te.loadReportData()
	if err != nil {
		return "", err
	}
	provider, ok := report["provider"].(map[string]interface{})
	if !ok {
		return "[]", nil
	}
	plugins, ok := provider["plugins"].(map[string]interface{})
	if !ok {
		return "[]", nil
	}
	pluginData, ok := plugins[params.PluginName].(map[string]interface{})
	if !ok {
		return "[]", nil
	}
	tests, ok := pluginData["tests"].(map[string]interface{})
	if !ok {
		return "[]", nil
	}
	var failed []map[string]interface{}
	for name, t := range tests {
		test, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		status, _ := test["status"].(string)
		if status == "failed" {
			failed = append(failed, map[string]interface{}{
				"id":     test["id"],
				"name":   name,
				"status": status,
				"state":  test["state"],
				"errors": test["errors"],
			})
		}
	}
	return marshalResult(failed)
}

func (te *ToolExecutor) readTestFailureLog(input json.RawMessage) (string, error) {
	var params TestIDInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	return te.readTestFile(params.TestID, "failure.txt")
}

func (te *ToolExecutor) readTestOutput(input json.RawMessage) (string, error) {
	var params TestIDInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	return te.readTestFile(params.TestID, "systemOut.txt")
}

func (te *ToolExecutor) readTestFile(testID, suffix string) (string, error) {
	parts := strings.SplitN(testID, "-", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid test ID format: %s", testID)
	}
	pluginPrefix := parts[0]

	entries, err := os.ReadDir(te.reportDir)
	if err != nil {
		return "", fmt.Errorf("failed to read report dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "failures-") {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "failures-"+pluginPrefix+"-") {
			continue
		}
		filePath := filepath.Join(te.reportDir, entry.Name(), testID+"-"+suffix)
		data, err := os.ReadFile(filePath)
		if err == nil {
			content := string(data)
			if len(content) > 50000 {
				content = content[:50000] + "\n... [truncated]"
			}
			return content, nil
		}
	}

	return "", fmt.Errorf("failure log not found for test %s", testID)
}

func (te *ToolExecutor) readEtcdData() (string, error) {
	report, err := te.loadReportData()
	if err != nil {
		return "", err
	}
	provider, ok := report["provider"].(map[string]interface{})
	if !ok {
		return "{}", nil
	}
	mgInfo, ok := provider["mustGatherInfo"].(map[string]interface{})
	if !ok {
		return "{}", nil
	}
	result := map[string]interface{}{}
	if v, ok := mgInfo["ErrorEtcdLogs"]; ok {
		result["errorEtcdLogs"] = v
	}
	return marshalResult(result)
}

func (te *ToolExecutor) readNetworkData() (string, error) {
	report, err := te.loadReportData()
	if err != nil {
		return "", err
	}
	provider, ok := report["provider"].(map[string]interface{})
	if !ok {
		return "{}", nil
	}
	mgInfo, ok := provider["mustGatherInfo"].(map[string]interface{})
	if !ok {
		return "{}", nil
	}
	result := map[string]interface{}{}
	if v, ok := mgInfo["PodNetworkChecks"]; ok {
		result["podNetworkChecks"] = v
	}
	return marshalResult(result)
}

func marshalResult(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}
