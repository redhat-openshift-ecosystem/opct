/*
Handle items in the file path meta/config.json
*/
package archive

import (
	sbconfig "github.com/vmware-tanzu/sonobuoy/pkg/config"
)

// MetaConfigSonobuoy is the sonobuoy configuration type.
type MetaConfigSonobuoy = sbconfig.Config

// ParseMetaConfig extract relevant attributes to export to data keeper.
func ParseMetaConfig(cfg *MetaConfigSonobuoy) []*RuntimeInfoItem {
	var runtimeConfig []*RuntimeInfoItem

	// General Server
	runtimeConfig = append(runtimeConfig, &RuntimeInfoItem{Name: "UUID", Value: cfg.UUID})
	runtimeConfig = append(runtimeConfig, &RuntimeInfoItem{Name: "Version", Value: cfg.Version})
	runtimeConfig = append(runtimeConfig, &RuntimeInfoItem{Name: "ResultsDir", Value: cfg.ResultsDir})
	runtimeConfig = append(runtimeConfig, &RuntimeInfoItem{Name: "Namespace", Value: cfg.Namespace})

	// Plugins
	runtimeConfig = append(runtimeConfig, &RuntimeInfoItem{Name: "WorkerImage", Value: cfg.WorkerImage})
	runtimeConfig = append(runtimeConfig, &RuntimeInfoItem{Name: "ImagePullPolicy", Value: cfg.ImagePullPolicy})

	// Security Config (customized by OPCT)
	runtimeConfig = append(runtimeConfig, &RuntimeInfoItem{Name: "AggregatorPermissions", Value: cfg.AggregatorPermissions})
	runtimeConfig = append(runtimeConfig, &RuntimeInfoItem{Name: "ServiceAccountName", Value: cfg.ServiceAccountName})
	existingSA := "no"
	if cfg.ExistingServiceAccount {
		existingSA = "yes"
	}
	runtimeConfig = append(runtimeConfig, &RuntimeInfoItem{Name: "ExistingServiceAccount", Value: existingSA})
	runtimeConfig = append(runtimeConfig, &RuntimeInfoItem{Name: "SecurityContextMode", Value: cfg.SecurityContextMode})

	return runtimeConfig
}
