package summary

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/pkg/errors"
	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

// OpenShiftSummary holds the data collected from artifacts related to OpenShift objects.
type OpenShiftSummary struct {
	Infrastructure   *configv1.Infrastructure
	ClusterVersion   *configv1.ClusterVersion
	ClusterOperators *configv1.ClusterOperatorList
	ClusterNetwork   *configv1.Network
	Nodes            []*Node
	InstallInvoker   string

	// Plugin Results
	PluginResultK8sConformance     *plugin.OPCTPluginSummary
	PluginResultOCPValidated       *plugin.OPCTPluginSummary
	PluginResultConformanceUpgrade *plugin.OPCTPluginSummary
	PluginResultArtifactsCollector *plugin.OPCTPluginSummary
	PluginResultConformanceReplay  *plugin.OPCTPluginSummary

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
	HistoryCompletionTime             string `json:"historyCompletionTime,omitempty"`
}

type SummaryClusterOperatorOutput struct {
	CountAvailable   uint64
	CountProgressing uint64
	CountDegraded    uint64
}

type SummaryOpenShiftInfrastructureV1 = configv1.Infrastructure
type SummaryOpenShiftClusterNetworkV1 = configv1.Network
type SummaryOpenShiftNetworkV1 = configv1.Network

type Node struct {
	Hostname          string            `json:"hostname,omitempty"`
	Architecture      string            `json:"architecture,omitempty"`
	OperatingSystem   string            `json:"os,omitempty"`
	OperatingSystemId string            `json:"osId,omitempty"`
	CreationDate      string            `json:"creationDate,omitempty"`
	NodeRoles         string            `json:"nodeRoles,omitempty"`
	TaintsNodeRole    string            `json:"taints,omitempty"`
	CapacityCPU       string            `json:"capacityCpu,omitempty"`
	CapacityStorageGB string            `json:"capacityStorageGB,omitempty"`
	CapacityMemGB     string            `json:"capacityMemGB,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	ControlPlane      bool              `json:"controlPlane,omitempty"`
}

type SummaryOpenShiftConfig struct {
	InstallManifestInvoker string `json:"installManifestInvoker,omitempty"`
}

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

func (os *OpenShiftSummary) GetInfrastructurePlatformType() string {
	if os.Infrastructure == nil {
		return "None"
	}
	return string(os.Infrastructure.Status.PlatformStatus.Type)
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
	// Get the oldest completionTime as string
	oldestTime := time.Now()
	for _, hist := range os.ClusterVersion.Status.History {
		if hist.CompletionTime.Compare(oldestTime) == -1 {
			oldestTime = hist.CompletionTime.Time
		}
	}
	resp.HistoryCompletionTime = oldestTime.String()

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

func (os *OpenShiftSummary) GetNodes() []*Node {
	return os.Nodes
}

func (os *OpenShiftSummary) SetNodes(nodes *v1.NodeList) error {
	if len(nodes.Items) == 0 {
		return errors.New("Unable to find result Items to set Nodes")
	}
	sizeToHuman := func(size string) string {
		sizeNumber := strings.Split(size, "Ki")[0]
		sizeInteger, err := strconv.Atoi(sizeNumber)
		if err != nil {
			return size
		}
		return fmt.Sprintf("%.2f", float64((sizeInteger/1024)/1024))
	}
	for _, node := range nodes.Items {
		// transforming complex k8s type to cleaned structure.
		customNode := Node{
			// Hostname: node.Status.Addresses,
			CapacityCPU:       node.Status.Capacity.Cpu().String(),
			CapacityStorageGB: sizeToHuman(node.Status.Capacity.StorageEphemeral().String()),
			CapacityMemGB:     sizeToHuman(node.Status.Capacity.Memory().String()),
			CreationDate:      node.GetObjectMeta().GetCreationTimestamp().String(),
			Labels:            make(map[string]string),
		}
		// parse labels
		for label, value := range node.GetObjectMeta().GetLabels() {
			switch label {
			case "kubernetes.io/os":
				customNode.OperatingSystem = value
				continue
			case "kubernetes.io/hostname":
				customNode.Hostname = value
				continue
			case "kubernetes.io/arch":
				customNode.Architecture = value
				continue
			case "node.openshift.io/os_id":
				customNode.OperatingSystemId = value
				continue
			case "topology.kubernetes.io/zone":
				customNode.Labels["topology.kubernetes.io/zone"] = value
				continue
			}
			if strings.HasPrefix(label, "node-role.kubernetes.io") {
				if roleArr := strings.Split(label, "node-role.kubernetes.io/"); len(roleArr) == 2 {
					if roleArr[1] == "master" || roleArr[1] == "control-plane" {
						customNode.ControlPlane = true
					}
					customNode.NodeRoles += fmt.Sprintf("%s ", roleArr[1])
					continue
				}
			}
		}
		// parse taints
		for _, taint := range node.Spec.Taints {
			if strings.HasPrefix(taint.Key, "node-role") {
				customNode.TaintsNodeRole += fmt.Sprintf("%s:%s ", taint.Key, taint.Effect)
			}
		}
		os.Nodes = append(os.Nodes, &customNode)
	}
	return nil
}

func (os *OpenShiftSummary) SetPluginResult(in *plugin.OPCTPluginSummary) error {
	switch in.Name {
	// Kubernetes Conformance plugin
	case plugin.PluginNameKubernetesConformance:
		os.PluginResultK8sConformance = in
	case plugin.PluginOldNameKubernetesConformance:
		in.NameAlias = in.Name
		in.Name = plugin.PluginNameKubernetesConformance
		os.PluginResultK8sConformance = in

	// OpenShift Conformance plugin
	case plugin.PluginNameOpenShiftConformance:
		os.PluginResultOCPValidated = in
	case plugin.PluginOldNameOpenShiftConformance:
		in.NameAlias = in.Name
		in.Name = plugin.PluginOldNameOpenShiftConformance
		os.PluginResultOCPValidated = in

	// Other plugins
	case plugin.PluginNameOpenShiftUpgrade:
		os.PluginResultConformanceUpgrade = in
	case plugin.PluginNameArtifactsCollector:
		os.PluginResultArtifactsCollector = in
	case plugin.PluginNameConformanceReplay:
		os.PluginResultConformanceReplay = in
	default:
		// return fmt.Errorf("unable to Set Plugin results: Plugin not found: %s", in.Name)
		return nil
	}
	return nil
}

// ExtractOpenShiftConfigMap processes a list of ConfigMaps and extracts specific
// information related to OpenShift installation. It looks for a ConfigMap with
// the name "openshift-install-manifests" and retrieves the value of the "invoker"
// key from its data, storing it in the InstallInvoker field of the OpenShiftSummary
// struct.
//
// Parameters:
//   - in: A pointer to a v1.ConfigMapList containing the ConfigMaps to be processed.
//
// Returns:
//   - An error if any issues occur during processing.
//
// Notes:
//   - If the provided ConfigMapList is nil or contains no items, the function logs
//     a debug message and returns nil without performing any further processing.
func (os *OpenShiftSummary) ExtractOpenShiftConfigMap(in *v1.ConfigMapList) error {
	if in == nil || len(in.Items) == 0 {
		log.Debugf("unable to read OPCT config map. ConfigMapList not found: %v", in)
		return nil
	}

	// Extract desired data from ConfigMaps
	for _, cm := range in.Items {
		if cm.ObjectMeta.Name != "openshift-install-manifests" {
			continue
		}
		// Extract only required information: invoker
		if invoker, ok := cm.Data["invoker"]; ok {
			os.InstallInvoker = invoker
		}
		break
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
	if os.PluginResultConformanceUpgrade == nil {
		return &plugin.OPCTPluginSummary{}
	}
	return os.PluginResultConformanceUpgrade
}

func (os *OpenShiftSummary) GetResultArtifactsCollector() *plugin.OPCTPluginSummary {
	if os.PluginResultArtifactsCollector == nil {
		return &plugin.OPCTPluginSummary{}
	}
	return os.PluginResultArtifactsCollector
}

func (os *OpenShiftSummary) GetResultConformanceReplay() *plugin.OPCTPluginSummary {
	if os.PluginResultConformanceReplay == nil {
		return &plugin.OPCTPluginSummary{}
	}
	return os.PluginResultConformanceReplay
}
