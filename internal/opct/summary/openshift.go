package summary

import (
	"fmt"
	"regexp"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/pkg/errors"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/opct/plugin"
)

// OpenShiftSummary holds the data collected from artifacts related to OpenShift objects.
type OpenShiftSummary struct {
	Infrastructure   *configv1.Infrastructure
	ClusterVersion   *configv1.ClusterVersion
	ClusterOperators *configv1.ClusterOperatorList
	ClusterNetwork   *configv1.Network

	// Plugin Results
	PluginResultK8sConformance     *plugin.OPCTPluginSummary
	PluginResultOCPValidated       *plugin.OPCTPluginSummary
	PluginResultConformanceUpgrade *plugin.OPCTPluginSummary
	PluginResultArtifactsCollector *plugin.OPCTPluginSummary

	// get from Sonobuoy metadata
	VersionK8S string
}

type SummaryClusterVersionOutput struct {
	Desired                           string `json:"desired"`
	Previous                          string `json:"previous"`
	Channel                           string `json:"channel"`
	ClusterID                         string `json:"clusterID"`
	OverallStatus                     string `json:"overallStatus"`
	OverallStatusReason               string `json:"overallStatusReason,omitempty"`
	OverallStatusMessage              string `json:"overallStatusMessage,omitempty"`
	CondAvailable                     string `json:"conditionAvailable,omitempty"`
	CondFailing                       string `json:"conditionFailing,omitempty"`
	CondProgressing                   string `json:"conditionProgressing,omitempty"`
	CondProgressingMessage            string `json:"conditionProgressingMessage,omitempty"`
	CondRetrievedUpdates              string `json:"conditionUpdates,omitempty"`
	CondImplicitlyEnabledCapabilities string `json:"conditionImplicitlyEnabledCapabilities,omitempty"`
	CondReleaseAccepted               string `json:"conditionReleaseAccepted,omitempty"`
}

type SummaryClusterOperatorOutput struct {
	CountAvailable   uint64
	CountProgressing uint64
	CountDegraded    uint64
}

type SummaryOpenShiftInfrastructureV1 = configv1.Infrastructure
type SummaryOpenShiftClusterNetworkV1 = configv1.Network
type SummaryOpenShiftNetworkV1 = configv1.Network

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

func (os *OpenShiftSummary) GetClusterNetwork() (*SummaryOpenShiftClusterNetworkV1, error) {
	if os.Infrastructure == nil {
		return &SummaryOpenShiftClusterNetworkV1{}, nil
	}
	return os.ClusterNetwork, nil
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
		Desired:   os.ClusterVersion.Status.Desired.Version,
		Channel:   os.ClusterVersion.Spec.Channel,
		ClusterID: string(os.ClusterVersion.Spec.ClusterID),
	}
	for _, condition := range os.ClusterVersion.Status.Conditions {
		if condition.Type == configv1.OperatorProgressing {
			resp.CondProgressing = string(condition.Status)
			resp.CondProgressingMessage = condition.Message
			if string(condition.Status) == "True" {
				resp.OverallStatusReason = fmt.Sprintf("%sProgressing ", resp.OverallStatusReason)
			}
			continue
		}
		if string(condition.Type) == "ImplicitlyEnabledCapabilities" {
			resp.CondImplicitlyEnabledCapabilities = string(condition.Status)
			continue
		}
		if string(condition.Type) == "ReleaseAccepted" {
			resp.CondReleaseAccepted = string(condition.Status)
			continue
		}
		if string(condition.Type) == "Available" {
			resp.CondAvailable = string(condition.Status)
			if string(condition.Status) == "False" {
				resp.OverallStatus = "Unavailable"
				resp.OverallStatusReason = fmt.Sprintf("%sAvailable ", resp.OverallStatusReason)
				resp.OverallStatusMessage = condition.Message
			} else {
				resp.OverallStatus = string(condition.Type)
			}
			continue
		}
		if string(condition.Type) == "Failing" {
			resp.CondFailing = string(condition.Status)
			if string(condition.Status) == "True" {
				resp.OverallStatus = string(condition.Type)
				resp.OverallStatusReason = fmt.Sprintf("%sFailing ", resp.OverallStatusReason)
				resp.OverallStatusMessage = condition.Message
			}
			continue
		}
		if string(condition.Type) == "RetrievedUpdates" {
			resp.CondRetrievedUpdates = string(condition.Status)
			continue
		}
	}
	// TODO navigate through history and fill Previous
	resp.Previous = "TODO"
	return &resp, nil
}

func (os *OpenShiftSummary) GetClusterVersionXY() (string, error) {
	out, err := os.GetClusterVersion()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`^(\d+.\d+)`)
	match := re.FindStringSubmatch(out.Desired)
	return match[1], nil
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

func (os *OpenShiftSummary) SetClusterNetwork(cn *configv1.NetworkList) error {
	if len(cn.Items) == 0 {
		return errors.New("Unable to find result Items to set ClusterNetwork")
	}
	os.ClusterNetwork = &cn.Items[0]
	return nil
}

func (os *OpenShiftSummary) SetPluginResult(in *plugin.OPCTPluginSummary) error {
	switch in.Name {
	case plugin.PluginNameKubernetesConformance:
		os.PluginResultK8sConformance = in
	case plugin.PluginOldNameKubernetesConformance:
		in.NameAlias = in.Name
		in.Name = plugin.PluginNameKubernetesConformance
		os.PluginResultK8sConformance = in

	case plugin.PluginNameOpenShiftConformance:
		os.PluginResultOCPValidated = in
	case plugin.PluginOldNameOpenShiftConformance:
		in.NameAlias = in.Name
		in.Name = plugin.PluginOldNameOpenShiftConformance
		os.PluginResultOCPValidated = in

	case plugin.PluginNameOpenShiftUpgrade:
		os.PluginResultConformanceUpgrade = in
	case plugin.PluginNameArtifactsCollector:
		os.PluginResultArtifactsCollector = in
	default:
		// return fmt.Errorf("unable to Set Plugin results: Plugin not found: %s", in.Name)
		return nil
	}
	return nil
}

func (os *OpenShiftSummary) GetResultOCPValidated() *plugin.OPCTPluginSummary {
	return os.PluginResultOCPValidated
}

func (os *OpenShiftSummary) GetResultK8SValidated() *plugin.OPCTPluginSummary {
	return os.PluginResultK8sConformance
}

func (os *OpenShiftSummary) GetResultConformanceUpgrade() *plugin.OPCTPluginSummary {
	return os.PluginResultConformanceUpgrade
}

func (os *OpenShiftSummary) GetResultArtifactsCollector() *plugin.OPCTPluginSummary {
	return os.PluginResultArtifactsCollector
}
