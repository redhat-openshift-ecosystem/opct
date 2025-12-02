package run

import (
	"testing"

	"github.com/redhat-openshift-ecosystem/opct/pkg"
	kcorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

// TestValidateOpctNamespace tests the validateOpctNamespace function
func TestValidateOpctNamespace(t *testing.T) {
	tests := []struct {
		name         string
		existingPods []kcorev1.Namespace
		expectErrors bool
		errorCount   int
	}{
		{
			name:         "namespace does not exist",
			existingPods: []kcorev1.Namespace{},
			expectErrors: false,
			errorCount:   0,
		},
		{
			name: "namespace already exists",
			existingPods: []kcorev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: pkg.CertificationNamespace,
					},
				},
			},
			expectErrors: true,
			errorCount:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset with existing namespaces
			objects := make([]runtime.Object, len(tt.existingPods))
			for i := range tt.existingPods {
				objects[i] = &tt.existingPods[i]
			}

			kclient := fake.NewSimpleClientset(objects...)
			opts := newRunOptions()

			errs := validateOpctNamespace(opts, kclient.CoreV1())

			if tt.expectErrors && len(errs) == 0 {
				t.Errorf("validateOpctNamespace() expected errors but got none")
			}

			if !tt.expectErrors && len(errs) > 0 {
				t.Errorf("validateOpctNamespace() unexpected errors: %v", errs)
			}

			if tt.errorCount > 0 && len(errs) != tt.errorCount {
				t.Errorf("validateOpctNamespace() expected %d errors, got %d", tt.errorCount, len(errs))
			}
		})
	}
}

// TestValidateDedicatedNode tests the validateDedicatedNode function
func TestValidateDedicatedNode(t *testing.T) {
	tests := []struct {
		name         string
		dedicated    bool
		existingNodes []kcorev1.Node
		expectErrors bool
		errorMessage string
	}{
		{
			name:         "dedicated mode disabled",
			dedicated:    false,
			existingNodes: []kcorev1.Node{},
			expectErrors: false,
		},
		{
			name:      "dedicated mode enabled - no nodes with label",
			dedicated: true,
			existingNodes: []kcorev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
						},
					},
				},
			},
			expectErrors: true,
			errorMessage: "missing dedicated node",
		},
		{
			name:      "dedicated mode enabled - node with label but no taint",
			dedicated: true,
			existingNodes: []kcorev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
						Labels: map[string]string{
							pkg.DedicatedNodeRoleLabel: "",
						},
					},
					Spec: kcorev1.NodeSpec{
						Taints: []kcorev1.Taint{},
					},
				},
			},
			expectErrors: true,
			errorMessage: "missing taint",
		},
		{
			name:      "dedicated mode enabled - valid node with label and taint",
			dedicated: true,
			existingNodes: []kcorev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node",
						Labels: map[string]string{
							pkg.DedicatedNodeRoleLabel: "",
						},
					},
					Spec: kcorev1.NodeSpec{
						Taints: []kcorev1.Taint{
							{
								Key:    pkg.DedicatedNodeRoleLabel,
								Effect: kcorev1.TaintEffectNoSchedule,
							},
						},
					},
				},
			},
			expectErrors: false,
		},
		{
			name:      "dedicated mode enabled - too many nodes with label",
			dedicated: true,
			existingNodes: []kcorev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node-1",
						Labels: map[string]string{
							pkg.DedicatedNodeRoleLabel: "",
						},
					},
					Spec: kcorev1.NodeSpec{
						Taints: []kcorev1.Taint{
							{
								Key:    pkg.DedicatedNodeRoleLabel,
								Effect: kcorev1.TaintEffectNoSchedule,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node-2",
						Labels: map[string]string{
							pkg.DedicatedNodeRoleLabel: "",
						},
					},
					Spec: kcorev1.NodeSpec{
						Taints: []kcorev1.Taint{
							{
								Key:    pkg.DedicatedNodeRoleLabel,
								Effect: kcorev1.TaintEffectNoSchedule,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-node-3",
						Labels: map[string]string{
							pkg.DedicatedNodeRoleLabel: "",
						},
					},
					Spec: kcorev1.NodeSpec{
						Taints: []kcorev1.Taint{
							{
								Key:    pkg.DedicatedNodeRoleLabel,
								Effect: kcorev1.TaintEffectNoSchedule,
							},
						},
					},
				},
			},
			expectErrors: true,
			errorMessage: "too many nodes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake clientset with existing nodes
			objects := make([]runtime.Object, len(tt.existingNodes))
			for i := range tt.existingNodes {
				objects[i] = &tt.existingNodes[i]
			}

			kclient := fake.NewSimpleClientset(objects...)
			opts := newRunOptions()
			opts.dedicated = tt.dedicated

			errs := validateDedicatedNode(opts, kclient.CoreV1())

			if tt.expectErrors && len(errs) == 0 {
				t.Errorf("validateDedicatedNode() expected errors but got none")
			}

			if !tt.expectErrors && len(errs) > 0 {
				t.Errorf("validateDedicatedNode() unexpected errors: %v", errs)
			}

			if tt.errorMessage != "" && len(errs) > 0 {
				found := false
				for _, err := range errs {
					if err != nil && len(err.Error()) > 0 {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("validateDedicatedNode() expected error containing %q, got %v", tt.errorMessage, errs)
				}
			}
		})
	}
}
