/*
Handle items in the file path resources/ns/{opct_namespace}/core_v1_configmaps.json
*/
package archive

import (
	"fmt"
	"sort"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

// ParseOpctConfig is a method to parse a list of config maps,
// kubernetes resource, used by OPCT to send configuration from
// CLI to plugins in runtime, extracting relevant information
// about the runtime.
func ParseOpctConfig(cms *v1.ConfigMapList) []*RuntimeInfoItem {
	if cms == nil {
		log.Debugf("unable to read OPCT config map. ConfigMapList not found: %v", cms)
		return []*RuntimeInfoItem{}
	}
	if len(cms.Items) == 0 {
		log.Debugf("unable to read OPCT config map. ConfigMapList not found: %v", cms)
		return []*RuntimeInfoItem{}
	}

	cmpMap := make(map[string]*RuntimeInfoItem)
	var keys []string
	for _, cm := range cms.Items {
		switch cm.ObjectMeta.Name {
		case "openshift-provider-certification-version", "opct-version", "plugins-config":
		default:
			continue
		}

		for k, v := range cm.Data {
			key := fmt.Sprintf("%s_%s", cm.ObjectMeta.Name, k)
			cmpMap[key] = &RuntimeInfoItem{Config: cm.ObjectMeta.Name, Name: k, Value: v}
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return []*RuntimeInfoItem{}
	}

	// Keeping an ordered list is important to unit tests.
	sort.Strings(keys)
	cmData := make([]*RuntimeInfoItem, 0, len(keys))
	for _, key := range keys {
		cmData = append(cmData, cmpMap[key])
	}
	return cmData
}
