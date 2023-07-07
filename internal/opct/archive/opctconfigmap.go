/*
Handle items in the file path resources/ns/{opct_namespace}/core_v1_configmaps.json
*/
package archive

import (
	v1 "k8s.io/api/core/v1"
)

func OpctConfigMapNames() []string {
	return []string{
		"openshift-provider-certification-version",
		"plugins-config",
	}
}

func ParseOpctConfig(cms *v1.ConfigMapList) []*RuntimeInfoItem {
	var cmData []*RuntimeInfoItem
	for _, cm := range cms.Items {

		switch cm.ObjectMeta.Name {
		case "openshift-provider-certification-version", "plugins-config":
		default:
			continue
		}
		for k, v := range cm.Data {
			cmData = append(cmData, &RuntimeInfoItem{Config: cm.ObjectMeta.Name, Name: k, Value: v})
		}
	}
	return cmData
}
