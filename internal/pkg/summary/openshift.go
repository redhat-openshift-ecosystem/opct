package summary

import (
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/pkg/errors"
)

type OpenShiftSummary struct {
	Infrastructure   *configv1.Infrastructure
	ClusterVersion   *configv1.ClusterVersion
	ClusterOperators *configv1.ClusterOperatorList

	// Plugin Results
	PluginResultK8sConformance *OPCTPluginSummary
	PluginResultOCPValidated   *OPCTPluginSummary

	// get from Sonobuoy metadata
	VersionK8S string
}

type SummaryClusterVersionOutput struct {
	DesiredVersion     string
	Progressing        string
	ProgressingMessage string
}

type SummaryClusterOperatorOutput struct {
	CountAvailable   uint64
	CountProgressing uint64
	CountDegraded    uint64
}

type SummaryOpenShiftInfrastructureV1 = configv1.Infrastructure

func NewOpenShiftSummary() *OpenShiftSummary {
	return &OpenShiftSummary{}
}

func (os *OpenShiftSummary) SetInfrastructure(cr *configv1.InfrastructureList) error {
	if len(cr.Items) == 0 {
		return errors.New("Unable to find result Items to set Infrastructures")
	}
	os.Infrastructure = &cr.Items[0]
	return nil
}

func (os *OpenShiftSummary) GetInfrastructure() (*SummaryOpenShiftInfrastructureV1, error) {
	if os.Infrastructure == nil {
		return &SummaryOpenShiftInfrastructureV1{}, nil
	}
	return os.Infrastructure, nil
}

func (os *OpenShiftSummary) SetClusterVersion(cr *configv1.ClusterVersionList) error {
	if len(cr.Items) == 0 {
		return errors.New("Unable to find result Items to set Infrastructures")
	}
	os.ClusterVersion = &cr.Items[0]
	return nil
}

func (os *OpenShiftSummary) GetClusterVersion() (*SummaryClusterVersionOutput, error) {
	if os.ClusterVersion == nil {
		return &SummaryClusterVersionOutput{}, nil
	}
	resp := SummaryClusterVersionOutput{
		DesiredVersion: os.ClusterVersion.Status.Desired.Version,
	}
	for _, condition := range os.ClusterVersion.Status.Conditions {
		if condition.Type == configv1.OperatorProgressing {
			resp.Progressing = string(condition.Status)
			resp.ProgressingMessage = condition.Message
		}
	}
	return &resp, nil
}

func (os *OpenShiftSummary) SetClusterOperators(cr *configv1.ClusterOperatorList) error {
	if len(cr.Items) == 0 {
		return errors.New("Unable to find result Items to set ClusterOperators")
	}
	os.ClusterOperators = cr
	return nil
}

func (os *OpenShiftSummary) GetClusterOperator() (*SummaryClusterOperatorOutput, error) {
	out := SummaryClusterOperatorOutput{}
	for _, co := range os.ClusterOperators.Items {
		for _, condition := range co.Status.Conditions {
			switch condition.Type {
			case configv1.OperatorAvailable:
				if condition.Status == configv1.ConditionTrue {
					out.CountAvailable += 1
				}
			case configv1.OperatorProgressing:
				if condition.Status == configv1.ConditionTrue {
					out.CountProgressing += 1
				}
			case configv1.OperatorDegraded:
				if condition.Status == configv1.ConditionTrue {
					out.CountDegraded += 1
				}
			}
		}
	}
	return &out, nil
}

func (os *OpenShiftSummary) SetPluginResult(in *OPCTPluginSummary) error {
	switch in.Name {
	case PluginNameKubernetesConformance:
		os.PluginResultK8sConformance = in
	case PluginOldNameKubernetesConformance:
		in.NameAlias = in.Name
		in.Name = PluginNameKubernetesConformance
		os.PluginResultK8sConformance = in

	case PluginNameOpenShiftConformance:
		os.PluginResultOCPValidated = in
	case PluginOldNameOpenShiftConformance:
		in.NameAlias = in.Name
		in.Name = PluginOldNameOpenShiftConformance
		os.PluginResultOCPValidated = in
	default:
		return fmt.Errorf("unable to Set Plugin results: Plugin not found: %s", in.Name)
	}
	return nil
}

func (os *OpenShiftSummary) GetResultOCPValidated() *OPCTPluginSummary {
	return os.PluginResultOCPValidated
}

func (os *OpenShiftSummary) GetResultK8SValidated() *OPCTPluginSummary {
	return os.PluginResultK8sConformance
}
