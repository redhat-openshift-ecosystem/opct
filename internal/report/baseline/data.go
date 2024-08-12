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
				log.Debugf("BaselineAPI data for plugin %q is missing failures (failedPriority), skipping...", pluginName)
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
	// tags = obj["setup"].(map[string]interface{})["api"].(map[string]string)
	// fmt.Println(s)
	return tags, nil
}
