package pkg

const (
	AnyUIDClusterRoleBinding       = "opct-anyuid"
	PrivilegedClusterRoleBinding   = "opct-privileged"
	CertificationNamespace         = "openshift-provider-certification"
	VersionInfoConfigMapName       = "openshift-provider-certification-version"
	DedicatedNodeRoleLabel         = "node-role.kubernetes.io/tests"
	DedicatedNodeRoleLabelSelector = "node-role.kubernetes.io/tests="
	SonobuoyServiceAccountName     = "sonobuoy-serviceaccount"
	SonobuoyComponentLabelValue    = "sonobuoy"
)
