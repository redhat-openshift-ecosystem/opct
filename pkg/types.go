package pkg

const (
	PrivilegedClusterRole          = "opct-scc-privileged"
	PrivilegedClusterRoleBinding   = "opct-scc-privileged"
	CertificationNamespace         = "openshift-provider-certification"
	VersionInfoConfigMapName       = "openshift-provider-certification-version"
	PluginsVarsConfigMapName       = "plugins-config"
	DedicatedNodeRoleLabel         = "node-role.kubernetes.io/tests"
	DedicatedNodeRoleLabelSelector = "node-role.kubernetes.io/tests="
	SonobuoyServiceAccountName     = "sonobuoy-serviceaccount"
	SonobuoyLabelNamespaceName     = "namespace"
	SonobuoyLabelComponentName     = "component"
	SonobuoyLabelComponentValue    = "sonobuoy"
	DefaultToolsRepository         = "quay.io/ocp-cert"
	PluginsImage                   = "openshift-tests-provider-cert:v0.5.0-alpha.4"
)

var (
	SonobuoyDefaultLabels = map[string]string{
		SonobuoyLabelComponentName: SonobuoyLabelComponentValue,
		SonobuoyLabelNamespaceName: CertificationNamespace,
		// Enforcing privileged mode for PSA on Conformance/Sonobuoy environment.
		// https://issues.redhat.com/browse/OPCT-11
		// https://issues.redhat.com/browse/OPCT-31
		"pod-security.kubernetes.io/enforce": "privileged",
		"pod-security.kubernetes.io/audit":   "privileged",
		"pod-security.kubernetes.io/warn":    "privileged",
	}
)
