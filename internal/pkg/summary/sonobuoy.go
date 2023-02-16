package summary

import (
	"github.com/vmware-tanzu/sonobuoy/pkg/discovery"
)

type SonobuoySummary struct {
	Cluster *discovery.ClusterSummary
}

func (s *SonobuoySummary) SetCluster(c *discovery.ClusterSummary) error {
	s.Cluster = c
	return nil
}
