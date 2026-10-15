package summary

import (
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExtractOpenShiftConfigMap(t *testing.T) {
	t.Run("should_extract_invoker_from_valid_configmap", func(t *testing.T) {
		configMapList := &v1.ConfigMapList{
			Items: []v1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "openshift-install-manifests",
					},
					Data: map[string]string{
						"invoker": "user",
					},
				},
			},
		}

		osSummary := NewOpenShiftSummary()
		err := osSummary.ExtractOpenShiftConfigMap(configMapList)

		assert.NoError(t, err)
		assert.Equal(t, "user", osSummary.InstallInvoker)
	})

	t.Run("should_not_set_invoker_if_configmap_name_does_not_match", func(t *testing.T) {
		configMapList := &v1.ConfigMapList{
			Items: []v1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "other-configmap",
					},
					Data: map[string]string{
						"invoker": "test-invoker",
					},
				},
			},
		}

		osSummary := NewOpenShiftSummary()
		err := osSummary.ExtractOpenShiftConfigMap(configMapList)

		assert.NoError(t, err)
		assert.Empty(t, osSummary.InstallInvoker)
	})

	t.Run("should_handle_empty_configmap_list", func(t *testing.T) {
		configMapList := &v1.ConfigMapList{}

		osSummary := NewOpenShiftSummary()
		err := osSummary.ExtractOpenShiftConfigMap(configMapList)

		assert.NoError(t, err)
		assert.Empty(t, osSummary.InstallInvoker)
	})

	t.Run("should_handle_nil_configmap_list", func(t *testing.T) {
		osSummary := NewOpenShiftSummary()
		err := osSummary.ExtractOpenShiftConfigMap(nil)

		assert.NoError(t, err)
		assert.Empty(t, osSummary.InstallInvoker)
	})

	t.Run("should_handle_configmap_without_invoker_key", func(t *testing.T) {
		configMapList := &v1.ConfigMapList{
			Items: []v1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "openshift-install-manifests",
					},
					Data: map[string]string{
						"some-other-key": "some-value",
					},
				},
			},
		}

		osSummary := NewOpenShiftSummary()
		err := osSummary.ExtractOpenShiftConfigMap(configMapList)

		assert.NoError(t, err)
		assert.Empty(t, osSummary.InstallInvoker)
	})
}

func TestGetClusterVersionHistoryCompletionTime(t *testing.T) {
	completedAt := func(ts string) *metav1.Time {
		parsed, err := time.Parse("2006-01-02 15:04:05", ts)
		assert.NoError(t, err)
		return &metav1.Time{Time: parsed}
	}

	newSummary := func(history []configv1.UpdateHistory) *OpenShiftSummary {
		osSummary := NewOpenShiftSummary()
		osSummary.ClusterVersion = &configv1.ClusterVersion{
			Status: configv1.ClusterVersionStatus{
				History: history,
			},
		}
		return osSummary
	}

	t.Run("should_not_panic_with_partial_update_without_completion_time", func(t *testing.T) {
		osSummary := newSummary([]configv1.UpdateHistory{
			{Version: "4.21.37", State: configv1.CompletedUpdate, CompletionTime: completedAt("2026-10-14 08:00:00")},
			{Version: "4.22.18", State: configv1.PartialUpdate, CompletionTime: nil},
		})

		out, err := osSummary.GetClusterVersion()

		assert.NoError(t, err)
		assert.Contains(t, out.HistoryCompletionTime, "2026-10-14 08:00:00")
	})

	t.Run("should_return_oldest_completion_time", func(t *testing.T) {
		osSummary := newSummary([]configv1.UpdateHistory{
			{Version: "4.22.18", State: configv1.CompletedUpdate, CompletionTime: completedAt("2026-10-14 09:00:00")},
			{Version: "4.21.37", State: configv1.CompletedUpdate, CompletionTime: completedAt("2026-06-02 08:00:00")},
		})

		out, err := osSummary.GetClusterVersion()

		assert.NoError(t, err)
		assert.Contains(t, out.HistoryCompletionTime, "2026-06-02 08:00:00")
	})

	t.Run("should_leave_field_empty_when_no_history_has_completion_time", func(t *testing.T) {
		osSummary := newSummary([]configv1.UpdateHistory{
			{Version: "4.22.18", State: configv1.PartialUpdate, CompletionTime: nil},
		})

		out, err := osSummary.GetClusterVersion()

		assert.NoError(t, err)
		assert.Empty(t, out.HistoryCompletionTime)
	})

	t.Run("should_leave_field_empty_with_empty_history", func(t *testing.T) {
		osSummary := newSummary(nil)

		out, err := osSummary.GetClusterVersion()

		assert.NoError(t, err)
		assert.Empty(t, out.HistoryCompletionTime)
	})

	t.Run("should_return_empty_output_without_cluster_version", func(t *testing.T) {
		osSummary := NewOpenShiftSummary()

		out, err := osSummary.GetClusterVersion()

		assert.NoError(t, err)
		assert.Empty(t, out.HistoryCompletionTime)
	})
}
