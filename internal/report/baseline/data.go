package baseline

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// BaselineData is the struct that holds the baseline data. This struct exists
// to parse the ReportSummary retrieved from S3. The data is the same structure
// as the internal/report/data.go.ReportData, although it isn't possible to unmarshall
// while the cyclic dependencies isn't resolved between packages:
// - internal/report
// - internal/opct/summary
type BaselineData struct {
	raw []byte
}

func (bd *BaselineData) SetRawData(data []byte) {
	bd.raw = data
}

func (bd *BaselineData) GetRawData() []byte {
	return bd.raw
}

// GetPriorityFailuresFromPlugin returns the priority failures from a specific plugin.
// The priority failures are the failures that are marked as priority in the baseline
// report. It should be a temporary function while marshaling the data from the AP
// isn't possible.
func (bd *BaselineData) GetPriorityFailuresFromPlugin(pluginName string) ([]string, error) {
	failureStr := []string{}
	var baselineData map[string]interface{}
	err := json.Unmarshal(bd.raw, &baselineData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal baseline data: %w", err)
	}

	// cast the data extracting the plugin failures.
	for p := range baselineData["provider"].(map[string]interface{})["plugins"].(map[string]interface{}) {
		pluginBaseline := baselineData["provider"].(map[string]interface{})["plugins"].(map[string]interface{})[p]
		pluginID := pluginBaseline.(map[string]interface{})["id"]
		if pluginID != pluginName {
			continue
		}
		failures, ok := pluginBaseline.(map[string]interface{})["failedFiltered"]
		if !ok || failures == nil {
			failures, ok2 := pluginBaseline.(map[string]interface{})["failedPriority"]
			if !ok2 {
				log.Debugf("BaselineAPI data for plugin %q is missing failures (failedPriority), skipping...", pluginName)
				return failureStr, nil
			}
			if failures == nil {
				log.Debugf("BaselineAPI data for plugin %q is missing failures (failedFiltered), skipping...", pluginName)
				return failureStr, nil
			}
		}
		for _, f := range failures.([]interface{}) {
			failureStr = append(failureStr, f.(map[string]interface{})["name"].(string))
		}
	}
	return failureStr, nil
}

func (bd *BaselineData) GetSetupTags() (map[string]interface{}, error) {
	var tags map[string]interface{}
	var obj map[string]interface{}
	err := json.Unmarshal(bd.raw, &obj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal baseline data: %w", err)
	}

	fmt.Println(obj["setup"].(map[string]interface{}))
	tags = obj["setup"].(map[string]interface{})["api"].(map[string]interface{})
	return tags, nil
}

// GetWorkflow extracts the workflow field from the baseline summary payload.
// Returns the workflow string and true if the field was present (even if empty),
// or ("", false) when the payload predates the workflow field.
func (bd *BaselineData) GetWorkflow() (string, bool) {
	var obj map[string]interface{}
	if err := json.Unmarshal(bd.raw, &obj); err != nil {
		return "", false
	}
	setup, ok := obj["setup"].(map[string]interface{})
	if !ok {
		return "", false
	}
	api, ok := setup["api"].(map[string]interface{})
	if !ok {
		return "", false
	}
	wf, ok := api["workflow"]
	if !ok {
		// Legacy payload: field absent entirely.
		return "", false
	}
	s, _ := wf.(string)
	return s, true
}

// InferWorkflow attempts to positively determine the workflow from all
// available metadata in the baseline summary payload. It checks, in order:
//
//  1. setup.api.workflow — the explicit field added with workflow-scoped baselines.
//  2. provider.runtime.opctConfig run-mode — the original run-mode config
//     written by the OPCT runner (positive evidence from older payloads).
//  3. provider.isUpgradeWorkflow — a boolean set to true for upgrade runs;
//     only usable as positive evidence for "upgrade" (false is omitted by omitempty).
//
// Returns (workflow, true) when the workflow can be positively identified, or
// ("", false) when no metadata establishes it. Callers MUST NOT accept a
// payload whose workflow cannot be established when a specific workflow filter
// is active — doing so risks cross-workflow contamination.
func (bd *BaselineData) InferWorkflow() (string, bool) {
	// 1. Explicit workflow field — most authoritative.
	if wf, ok := bd.GetWorkflow(); ok {
		return wf, true
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(bd.raw, &obj); err != nil {
		return "", false
	}

	provider, _ := obj["provider"].(map[string]interface{})
	if provider == nil {
		return "", false
	}

	// 2. Check provider.runtime.opctConfig for run-mode entry.
	if runtime, ok := provider["runtime"].(map[string]interface{}); ok {
		if opctConfig, ok := runtime["opctConfig"].([]interface{}); ok {
			for _, item := range opctConfig {
				cfg, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if name, _ := cfg["name"].(string); name == "run-mode" {
					if v, _ := cfg["value"].(string); v != "" {
						return v, true
					}
				}
			}
		}
	}

	// 3. Check provider.isUpgradeWorkflow — only true is informative
	//    (false is omitted by omitempty in Go serialization).
	if isUpgrade, ok := provider["isUpgradeWorkflow"].(bool); ok && isUpgrade {
		return "upgrade", true
	}

	return "", false
}
