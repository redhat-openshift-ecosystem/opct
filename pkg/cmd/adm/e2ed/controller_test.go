package e2ed

import (
	"testing"

	types "github.com/redhat-openshift-ecosystem/opct/pkg"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newNode builds a node with the hostname label always set, optionally holding the dedicated taint.
func newNode(name string, dedicated bool, labels map[string]string) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"kubernetes.io/hostname": name},
		},
	}
	for k, v := range labels {
		node.Labels[k] = v
	}
	if dedicated {
		node.Labels[types.DedicatedNodeRoleLabel] = ""
		node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
			Key:    types.DedicatedNodeRoleLabel,
			Effect: corev1.TaintEffectNoSchedule,
		})
	}
	return node
}

// opctCluster mimics the OPCT environment: tainted control plane nodes, two regular workers and one
// worker reserved by "opct adm e2e-dedicated taint-node".
func opctCluster() []*corev1.Node {
	master := newNode("master-0", false, map[string]string{"node-role.kubernetes.io/master": ""})
	master.Spec.Taints = append(master.Spec.Taints, corev1.Taint{
		Key:    "node-role.kubernetes.io/master",
		Effect: corev1.TaintEffectNoSchedule,
	})
	return []*corev1.Node{
		master,
		newNode("worker-0", false, nil),
		newNode("worker-1", false, nil),
		newNode("worker-dedicated", true, nil),
	}
}

func podWithSelector(selector map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "e2e-test"},
		Spec:       corev1.PodSpec{NodeSelector: selector},
	}
}

func podWithRequiredAffinity(term corev1.NodeSelectorTerm) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "e2e-test"},
		Spec: corev1.PodSpec{
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{term},
					},
				},
			},
		},
	}
}

// matchFieldsOnNodeName reproduces the required node affinity the DaemonSet controller sets on the
// pods it creates for a given node.
func matchFieldsOnNodeName(nodeName string) corev1.NodeSelectorTerm {
	return corev1.NodeSelectorTerm{
		MatchFields: []corev1.NodeSelectorRequirement{{
			Key:      "metadata.name",
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{nodeName},
		}},
	}
}

func matchHostnames(hostnames ...string) corev1.NodeSelectorTerm {
	return corev1.NodeSelectorTerm{
		MatchExpressions: []corev1.NodeSelectorRequirement{{
			Key:      "kubernetes.io/hostname",
			Operator: corev1.NodeSelectorOpIn,
			Values:   hostnames,
		}},
	}
}

func TestIsPinnedToDedicatedNode(t *testing.T) {
	tests := []struct {
		name  string
		pod   *corev1.Pod
		nodes []*corev1.Node
		want  bool
	}{
		{
			// OPCT-461: the pod created by the SchedulerPredicates resource limit tests, which must
			// stay Pending. It has no placement constraint, so it is not pinned.
			name:  "unconstrained pod is not pinned",
			pod:   podWithSelector(nil),
			nodes: opctCluster(),
			want:  false,
		},
		{
			name:  "pod selecting the dedicated node by hostname is pinned",
			pod:   podWithSelector(map[string]string{"kubernetes.io/hostname": "worker-dedicated"}),
			nodes: opctCluster(),
			want:  true,
		},
		{
			name:  "pod selecting the dedicated node by role label is pinned",
			pod:   podWithSelector(map[string]string{types.DedicatedNodeRoleLabel: ""}),
			nodes: opctCluster(),
			want:  true,
		},
		{
			name:  "pod selecting a regular worker is not pinned",
			pod:   podWithSelector(map[string]string{"kubernetes.io/hostname": "worker-0"}),
			nodes: opctCluster(),
			want:  false,
		},
		{
			// A DaemonSet pod targets a single node through the required node affinity set by the
			// DaemonSet controller.
			name:  "pod with required affinity on the dedicated node is pinned",
			pod:   podWithRequiredAffinity(matchFieldsOnNodeName("worker-dedicated")),
			nodes: opctCluster(),
			want:  true,
		},
		{
			name:  "pod with required affinity spanning several nodes is not pinned",
			pod:   podWithRequiredAffinity(matchHostnames("worker-0", "worker-dedicated")),
			nodes: opctCluster(),
			want:  false,
		},
		{
			name: "pod bound to the dedicated node by name is pinned",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "e2e-test"},
				Spec:       corev1.PodSpec{NodeName: "worker-dedicated"},
			},
			nodes: opctCluster(),
			want:  true,
		},
		{
			name: "pod bound to a regular worker by name is not pinned",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "e2e-test"},
				Spec:       corev1.PodSpec{NodeName: "worker-0"},
			},
			nodes: opctCluster(),
			want:  false,
		},
		{
			name:  "pod selecting a label no node has is not pinned",
			pod:   podWithSelector(map[string]string{"kubernetes.io/hostname": "worker-gone"}),
			nodes: opctCluster(),
			want:  false,
		},
		{
			// The controller only injects a NoSchedule toleration, so a pod pinned to a node
			// tainted with NoExecute would stay unschedulable after the mutation.
			name:  "pod pinned to a node tainted with NoExecute is not pinned",
			pod:   podWithSelector(map[string]string{"kubernetes.io/hostname": "worker-noexecute"}),
			nodes: append(opctCluster(), newNodeTaintedWith("worker-noexecute", corev1.TaintEffectNoExecute)),
			want:  false,
		},
		{
			name:  "cluster without a dedicated node never pins",
			pod:   podWithSelector(map[string]string{"kubernetes.io/hostname": "worker-0"}),
			nodes: []*corev1.Node{newNode("worker-0", false, nil)},
			want:  false,
		},
		{
			name:  "empty node cache never pins",
			pod:   podWithSelector(map[string]string{"kubernetes.io/hostname": "worker-dedicated"}),
			nodes: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPinnedToDedicatedNode(tt.pod, tt.nodes); got != tt.want {
				t.Errorf("isPinnedToDedicatedNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

// newNodeTaintedWith builds a node holding the dedicated node key with an arbitrary taint effect.
func newNodeTaintedWith(name string, effect corev1.TaintEffect) *corev1.Node {
	node := newNode(name, false, map[string]string{types.DedicatedNodeRoleLabel: ""})
	node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
		Key:    types.DedicatedNodeRoleLabel,
		Effect: effect,
	})
	return node
}

func TestDedicatedNodeNames(t *testing.T) {
	nodes := opctCluster()
	// A node labeled but not tainted is not reserved yet, only the taint blocks scheduling.
	nodes = append(nodes, newNode("worker-labeled", false, map[string]string{types.DedicatedNodeRoleLabel: ""}))
	// The injected toleration only matches NoSchedule, so the other effects are not handled here.
	nodes = append(nodes,
		newNodeTaintedWith("worker-noexecute", corev1.TaintEffectNoExecute),
		newNodeTaintedWith("worker-prefernoschedule", corev1.TaintEffectPreferNoSchedule),
	)

	dedicated := dedicatedNodeNames(nodes)
	if len(dedicated) != 1 {
		t.Fatalf("dedicatedNodeNames() returned %d nodes, want 1: %v", len(dedicated), dedicated)
	}
	if _, ok := dedicated["worker-dedicated"]; !ok {
		t.Errorf("dedicatedNodeNames() = %v, want worker-dedicated", dedicated)
	}
}
