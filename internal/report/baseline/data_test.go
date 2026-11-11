package baseline

import (
	"encoding/json"
	"testing"
)

// makePayloadWithWorkflow builds a minimal summary payload with or without
// the setup.api.workflow field, mirroring what the CloudFront/S3 API returns.
func makePayloadWithWorkflow(workflow string) []byte {
	api := map[string]interface{}{
		"openshiftRelease": "4.20",
		"platformType":     "External",
		"executionDate":    "2026-01-01T00:00:00Z",
		"dataPath":         "4.20_External_20260101.json",
	}
	if workflow != "" {
		api["workflow"] = workflow
	}
	obj := map[string]interface{}{
		"setup": map[string]interface{}{"api": api},
		"provider": map[string]interface{}{
			"plugins": map[string]interface{}{},
		},
	}
	b, _ := json.Marshal(obj)
	return b
}

// makePayloadLegacy builds a payload with no workflow field at all.
func makePayloadLegacy() []byte {
	return makePayloadWithWorkflow("")
}

func TestBaselineData_GetWorkflow_Present(t *testing.T) {
	bd := &BaselineData{}
	bd.SetRawData(makePayloadWithWorkflow("upgrade"))

	wf, ok := bd.GetWorkflow()
	if !ok {
		t.Fatal("expected GetWorkflow to return ok=true for payload with workflow field")
	}
	if wf != "upgrade" {
		t.Errorf("expected workflow 'upgrade', got %q", wf)
	}
}

func TestBaselineData_GetWorkflow_Regular(t *testing.T) {
	bd := &BaselineData{}
	bd.SetRawData(makePayloadWithWorkflow("regular"))

	wf, ok := bd.GetWorkflow()
	if !ok {
		t.Fatal("expected GetWorkflow to return ok=true")
	}
	if wf != "regular" {
		t.Errorf("expected workflow 'regular', got %q", wf)
	}
}

func TestBaselineData_GetWorkflow_Legacy(t *testing.T) {
	bd := &BaselineData{}
	bd.SetRawData(makePayloadLegacy())

	wf, ok := bd.GetWorkflow()
	if ok {
		t.Fatalf("expected GetWorkflow to return ok=false for legacy payload, got workflow=%q", wf)
	}
	if wf != "" {
		t.Errorf("expected empty workflow for legacy, got %q", wf)
	}
}

func TestBaselineData_GetWorkflow_MalformedJSON(t *testing.T) {
	bd := &BaselineData{}
	bd.SetRawData([]byte("not json"))

	wf, ok := bd.GetWorkflow()
	if ok {
		t.Fatal("expected ok=false for malformed JSON")
	}
	if wf != "" {
		t.Errorf("expected empty workflow for malformed JSON, got %q", wf)
	}
}

func TestBaselineData_GetWorkflow_MissingSetup(t *testing.T) {
	bd := &BaselineData{}
	bd.SetRawData([]byte(`{"provider": {}}`))

	wf, ok := bd.GetWorkflow()
	if ok {
		t.Fatal("expected ok=false when setup block is missing")
	}
	if wf != "" {
		t.Errorf("expected empty workflow, got %q", wf)
	}
}

// --- InferWorkflow tests ---

// makePayloadWithLegacyMetadata builds a payload WITHOUT the explicit
// setup.api.workflow field but WITH positive metadata in the provider
// section (runtime.opctConfig run-mode and/or isUpgradeWorkflow).
func makePayloadWithLegacyMetadata(runMode string, isUpgrade bool) []byte {
	api := map[string]interface{}{
		"openshiftRelease": "4.20",
		"platformType":     "External",
		"executionDate":    "2026-01-01T00:00:00Z",
		"dataPath":         "4.20_External_20260101.json",
		// No "workflow" field — simulating a legacy payload.
	}

	provider := map[string]interface{}{
		"plugins": map[string]interface{}{},
	}
	if runMode != "" {
		provider["runtime"] = map[string]interface{}{
			"opctConfig": []interface{}{
				map[string]interface{}{"name": "run-mode", "value": runMode},
			},
		}
	}
	if isUpgrade {
		provider["isUpgradeWorkflow"] = true
	}

	obj := map[string]interface{}{
		"setup":    map[string]interface{}{"api": api},
		"provider": provider,
	}
	b, _ := json.Marshal(obj)
	return b
}

func TestInferWorkflow_ExplicitField(t *testing.T) {
	bd := &BaselineData{}
	bd.SetRawData(makePayloadWithWorkflow("upgrade"))

	wf, ok := bd.InferWorkflow()
	if !ok {
		t.Fatal("expected InferWorkflow to succeed for explicit workflow field")
	}
	if wf != "upgrade" {
		t.Errorf("expected 'upgrade', got %q", wf)
	}
}

func TestInferWorkflow_FromRunModeConfig(t *testing.T) {
	bd := &BaselineData{}
	bd.SetRawData(makePayloadWithLegacyMetadata("upgrade", false))

	wf, ok := bd.InferWorkflow()
	if !ok {
		t.Fatal("expected InferWorkflow to succeed from run-mode config")
	}
	if wf != "upgrade" {
		t.Errorf("expected 'upgrade', got %q", wf)
	}
}

func TestInferWorkflow_FromRunModeConfigRegular(t *testing.T) {
	bd := &BaselineData{}
	bd.SetRawData(makePayloadWithLegacyMetadata("regular", false))

	wf, ok := bd.InferWorkflow()
	if !ok {
		t.Fatal("expected InferWorkflow to succeed from run-mode=regular config")
	}
	if wf != "regular" {
		t.Errorf("expected 'regular', got %q", wf)
	}
}

func TestInferWorkflow_FromIsUpgradeWorkflow(t *testing.T) {
	// No run-mode config, but isUpgradeWorkflow=true.
	bd := &BaselineData{}
	bd.SetRawData(makePayloadWithLegacyMetadata("", true))

	wf, ok := bd.InferWorkflow()
	if !ok {
		t.Fatal("expected InferWorkflow to succeed from isUpgradeWorkflow=true")
	}
	if wf != "upgrade" {
		t.Errorf("expected 'upgrade', got %q", wf)
	}
}

func TestInferWorkflow_NoEvidence(t *testing.T) {
	// Legacy payload with no workflow field, no run-mode, no isUpgradeWorkflow.
	bd := &BaselineData{}
	bd.SetRawData(makePayloadLegacy())

	wf, ok := bd.InferWorkflow()
	if ok {
		t.Fatalf("expected InferWorkflow to return false for legacy payload without evidence, got workflow=%q", wf)
	}
	if wf != "" {
		t.Errorf("expected empty workflow, got %q", wf)
	}
}

func TestInferWorkflow_MalformedJSON(t *testing.T) {
	bd := &BaselineData{}
	bd.SetRawData([]byte("not json"))

	wf, ok := bd.InferWorkflow()
	if ok {
		t.Fatal("expected ok=false for malformed JSON")
	}
	if wf != "" {
		t.Errorf("expected empty workflow, got %q", wf)
	}
}

func TestInferWorkflow_NoProvider(t *testing.T) {
	bd := &BaselineData{}
	bd.SetRawData([]byte(`{"setup":{"api":{}}}`))

	wf, ok := bd.InferWorkflow()
	if ok {
		t.Fatal("expected ok=false when provider block is missing")
	}
	if wf != "" {
		t.Errorf("expected empty workflow, got %q", wf)
	}
}

// TestInferWorkflow_NonCanonicalRunMode verifies that a non-canonical
// run-mode value is returned as-is (caller comparison will reject it)
// but does NOT silently match "regular" or "upgrade".
func TestInferWorkflow_NonCanonicalRunMode(t *testing.T) {
	bd := &BaselineData{}
	bd.SetRawData(makePayloadWithLegacyMetadata("IfNotPresent", false))

	wf, ok := bd.InferWorkflow()
	if !ok {
		t.Fatal("expected InferWorkflow to return ok=true (run-mode was present)")
	}
	if wf != "IfNotPresent" {
		t.Errorf("expected raw value 'IfNotPresent', got %q", wf)
	}
	// Verify it is NOT canonical — callers must reject this.
	if IsCanonicalWorkflow(wf) {
		t.Error("IfNotPresent must NOT be treated as a canonical workflow")
	}
}

func TestInferWorkflow_ConflictingEvidence(t *testing.T) {
	// Explicit workflow field takes precedence even when
	// isUpgradeWorkflow contradicts it.
	api := map[string]interface{}{
		"openshiftRelease": "4.20",
		"platformType":     "External",
		"executionDate":    "2026-01-01T00:00:00Z",
		"dataPath":         "4.20_External_20260101.json",
		"workflow":         "regular",
	}
	provider := map[string]interface{}{
		"plugins":           map[string]interface{}{},
		"isUpgradeWorkflow": true,
	}
	obj := map[string]interface{}{
		"setup":    map[string]interface{}{"api": api},
		"provider": provider,
	}
	b, _ := json.Marshal(obj)

	bd := &BaselineData{}
	bd.SetRawData(b)
	wf, ok := bd.InferWorkflow()
	if !ok {
		t.Fatal("expected InferWorkflow to succeed")
	}
	if wf != "regular" {
		t.Errorf("explicit workflow 'regular' should take precedence, got %q", wf)
	}
}

// --- IsCanonicalWorkflow tests ---

func TestIsCanonicalWorkflow(t *testing.T) {
	tests := []struct {
		wf   string
		want bool
	}{
		{"regular", true},
		{"upgrade", true},
		{"", false},
		{"IfNotPresent", false},
		{"REGULAR", false},
		{"Upgrade", false},
		{"conformance", false},
	}
	for _, tc := range tests {
		got := IsCanonicalWorkflow(tc.wf)
		if got != tc.want {
			t.Errorf("IsCanonicalWorkflow(%q) = %v, want %v", tc.wf, got, tc.want)
		}
	}
}
