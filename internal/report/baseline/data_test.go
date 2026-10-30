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
