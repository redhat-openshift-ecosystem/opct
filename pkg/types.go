package pkg

const (
	PrivilegedClusterRole          = "opct-scc-privileged"
	PrivilegedClusterRoleBinding   = "opct-scc-privileged"
	CertificationNamespace         = "openshift-provider-certification"
	VersionInfoConfigMapName       = "openshift-provider-certification-version"
	DedicatedNodeRoleLabel         = "node-role.kubernetes.io/tests"
	DedicatedNodeRoleLabelSelector = "node-role.kubernetes.io/tests="
	SonobuoyServiceAccountName     = "sonobuoy-serviceaccount"
	SonobuoyLabelNamespaceName     = "namespace"
	SonobuoyLabelComponentName     = "component"
	SonobuoyLabelComponentValue    = "sonobuoy"
)

var (
	SonobuoyDefaultLabels = map[string]string{
		SonobuoyLabelComponentName: SonobuoyLabelComponentValue,
		SonobuoyLabelNamespaceName: CertificationNamespace,
	}
)
