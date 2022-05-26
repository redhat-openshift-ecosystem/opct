package destroy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/openshift/provider-certification-tool/pkg"
)

func Test_DeleteTestNamespaces(t *testing.T) {
	tests := []struct {
		name               string
		namespaces         []string
		expectedNamespaces []string
	}{{
		"remove e2e namespace",
		[]string{"openshift-cluster-version", "mytestns", "e2e-csi"},
		[]string{"openshift-cluster-version", "mytestns"},
	}, {
		"remove no namespaces",
		[]string{"openshift-cluster-version", "mytestns", "openshift"},
		[]string{"openshift-cluster-version", "openshift", "mytestns"},
	}}

	for _, test := range tests {
		// Setup initial namespaces
		var namespaces []runtime.Object
		for _, ns := range test.namespaces {
			namespaces = append(namespaces, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			})
		}

		// Get Clientset and initialize with namespaces
		clientset := fake.NewSimpleClientset(namespaces...)
		d := DestroyOptions{
			config: &pkg.Config{
				Clientset: clientset,
			},
		}

		// Delete namespaces
		err := d.DeleteTestNamespaces()
		assert.Nil(t, err)

		// Get actual list of namespaces after deletion
		result, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		assert.Nil(t, err)

		// Compare results
		for _, ns := range test.expectedNamespaces {
			found := false
			for _, actualNs := range result.Items {
				if ns == actualNs.Name {
					found = true
				}
			}
			assert.True(t, found, "could not find expected namespaces")
		}
	}
}
