package baseline

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetLatestRawSummaryFromPlatformWithFallback_WorkflowValidation
// uses an HTTP test server to simulate the CloudFront API and verifies
// that the consumer-path fallback correctly validates payload workflows
// using InferWorkflow (explicit field, run-mode config, isUpgradeWorkflow).
func TestGetLatestRawSummaryFromPlatformWithFallback_WorkflowValidation(t *testing.T) {
	// payloadSpec describes what the server should serve for a given path.
	type payloadSpec struct {
		// workflow sets the explicit setup.api.workflow field ("" = omit).
		workflow string
		// runMode sets provider.runtime.opctConfig run-mode ("" = omit).
		runMode string
		// isUpgrade sets provider.isUpgradeWorkflow to true.
		isUpgrade bool
	}

	tests := []struct {
		name           string
		requestWF      string                 // workflow requested by the caller
		servedPaths    map[string]payloadSpec // path -> payload spec (absent = 404)
		expectBuffer   bool
		expectWorkflow string // expected inferred workflow in the accepted payload
	}{
		{
			name:      "exact workflow-scoped match is accepted",
			requestWF: "upgrade",
			servedPaths: map[string]payloadSpec{
				"/result/summary/4.20_External_upgrade_latest.json": {workflow: "upgrade"},
			},
			expectBuffer:   true,
			expectWorkflow: "upgrade",
		},
		{
			name:      "wrong-workflow payload is rejected even if HTTP succeeds",
			requestWF: "upgrade",
			servedPaths: map[string]payloadSpec{
				"/result/summary/4.20_External_latest.json": {workflow: "regular"},
				"/result/summary/4.20_None_latest.json":     {workflow: "regular"},
				"/result/summary/4.20_AWS_latest.json":      {workflow: "regular"},
			},
			expectBuffer: false,
		},
		{
			name:      "legacy payload without ANY workflow evidence is rejected for upgrade target",
			requestWF: "upgrade",
			servedPaths: map[string]payloadSpec{
				// Payload has NO workflow field, NO run-mode, NO isUpgradeWorkflow.
				"/result/summary/4.20_External_latest.json": {},
			},
			expectBuffer: false, // must NOT accept unverifiable payload
		},
		{
			name:      "legacy payload without ANY workflow evidence is rejected for regular target",
			requestWF: "regular",
			servedPaths: map[string]payloadSpec{
				"/result/summary/4.20_External_latest.json": {},
			},
			expectBuffer: false, // must NOT accept unverifiable payload
		},
		{
			name:      "legacy with matching run-mode config is accepted for upgrade target",
			requestWF: "upgrade",
			servedPaths: map[string]payloadSpec{
				"/result/summary/4.20_External_latest.json": {runMode: "upgrade", isUpgrade: true},
			},
			expectBuffer:   true,
			expectWorkflow: "upgrade",
		},
		{
			name:      "legacy with matching run-mode config is accepted for regular target",
			requestWF: "regular",
			servedPaths: map[string]payloadSpec{
				"/result/summary/4.20_External_latest.json": {runMode: "regular"},
			},
			expectBuffer:   true,
			expectWorkflow: "regular",
		},
		{
			name:      "legacy with mismatched run-mode is rejected",
			requestWF: "upgrade",
			servedPaths: map[string]payloadSpec{
				"/result/summary/4.20_External_latest.json": {runMode: "regular"},
			},
			expectBuffer: false,
		},
		{
			name:      "legacy with isUpgradeWorkflow=true accepted for upgrade target",
			requestWF: "upgrade",
			servedPaths: map[string]payloadSpec{
				// Only isUpgradeWorkflow flag, no run-mode config.
				"/result/summary/4.20_External_latest.json": {isUpgrade: true},
			},
			expectBuffer:   true,
			expectWorkflow: "upgrade",
		},
		{
			name:      "no workflow filter accepts first available payload",
			requestWF: "",
			servedPaths: map[string]payloadSpec{
				"/result/summary/4.20_External_latest.json": {workflow: "upgrade"},
			},
			expectBuffer:   true,
			expectWorkflow: "upgrade",
		},
		{
			name:      "workflow-scoped path preferred over legacy with wrong workflow",
			requestWF: "upgrade",
			servedPaths: map[string]payloadSpec{
				"/result/summary/4.20_External_upgrade_latest.json": {workflow: "upgrade"},
				"/result/summary/4.20_External_latest.json":         {workflow: "regular"},
			},
			expectBuffer:   true,
			expectWorkflow: "upgrade",
		},
		{
			name:         "all paths 404 leaves buffer nil",
			requestWF:    "upgrade",
			servedPaths:  map[string]payloadSpec{},
			expectBuffer: false,
		},
		{
			name:      "platform fallback finds matching workflow on None",
			requestWF: "upgrade",
			servedPaths: map[string]payloadSpec{
				"/result/summary/4.20_None_upgrade_latest.json": {workflow: "upgrade"},
			},
			expectBuffer:   true,
			expectWorkflow: "upgrade",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				spec, ok := tt.servedPaths[r.URL.Path]
				if !ok {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(makePayloadForSpec(spec))
			}))
			defer srv.Close()

			origURL := reportBaseURL
			setReportBaseURL(srv.URL)
			defer setReportBaseURL(origURL)

			brs := NewBaselineReportSummary()
			err := brs.GetLatestRawSummaryFromPlatformWithFallback("4.20", "External", tt.requestWF)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			buf := brs.GetBuffer()
			if tt.expectBuffer && buf == nil {
				t.Fatal("expected buffer to be set, got nil")
			}
			if !tt.expectBuffer && buf != nil {
				t.Fatal("expected buffer to be nil (rejected), but it was set")
			}
			if buf != nil && tt.expectWorkflow != "" {
				wf, ok := buf.InferWorkflow()
				if !ok {
					t.Fatal("expected workflow to be inferable in accepted payload")
				}
				if wf != tt.expectWorkflow {
					t.Errorf("expected accepted payload workflow %q, got %q", tt.expectWorkflow, wf)
				}
			}
		})
	}
}

// makePayloadForSpec builds a JSON payload matching the given payloadSpec.
// If spec.workflow is set, it uses the explicit setup.api.workflow field.
// If spec.runMode or spec.isUpgrade are set, they populate legacy metadata.
// If all fields are zero-value, the payload has no workflow evidence at all.
func makePayloadForSpec(spec struct {
	workflow  string
	runMode   string
	isUpgrade bool
}) []byte {
	if spec.workflow != "" {
		return makePayloadWithWorkflow(spec.workflow)
	}
	if spec.runMode != "" || spec.isUpgrade {
		return makePayloadWithLegacyMetadata(spec.runMode, spec.isUpgrade)
	}
	// Bare legacy payload — no workflow evidence.
	return makePayloadLegacy()
}

// TestGetLatestSummaryByPlatform_RejectsWrongWorkflow tests that the
// explicit get-by-platform path rejects a legacy payload carrying a
// conflicting workflow.
func TestGetLatestSummaryByPlatform_RejectsWrongWorkflow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/result/summary/4.20_External_latest.json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(makePayloadWithWorkflow("regular"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	origURL := reportBaseURL
	setReportBaseURL(srv.URL)
	defer setReportBaseURL(origURL)

	brs := NewBaselineReportSummary()
	err := brs.GetLatestSummaryByPlatform("4.20", "External", "upgrade")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brs.GetBuffer() != nil {
		t.Error("expected buffer to be nil (wrong workflow payload should be rejected)")
	}
}

// TestGetLatestSummaryByPlatform_RejectsUnknownLegacy tests that the
// get-by-platform path rejects a legacy payload with no verifiable
// workflow metadata when a workflow filter is active.
func TestGetLatestSummaryByPlatform_RejectsUnknownLegacy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/result/summary/4.20_External_latest.json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Bare legacy payload, no workflow evidence at all.
			_, _ = w.Write(makePayloadLegacy())
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	origURL := reportBaseURL
	setReportBaseURL(srv.URL)
	defer setReportBaseURL(origURL)

	// Test rejection for upgrade target.
	brs := NewBaselineReportSummary()
	err := brs.GetLatestSummaryByPlatform("4.20", "External", "upgrade")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brs.GetBuffer() != nil {
		t.Error("expected buffer to be nil (unknown legacy should be rejected for upgrade target)")
	}

	// Test rejection for regular target.
	brs2 := NewBaselineReportSummary()
	err = brs2.GetLatestSummaryByPlatform("4.20", "External", "regular")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brs2.GetBuffer() != nil {
		t.Error("expected buffer to be nil (unknown legacy should be rejected for regular target)")
	}
}

// TestGetLatestSummaryByPlatform_AcceptsLegacyWithMatchingRunMode tests that
// legacy payloads with positive run-mode metadata are accepted when they match.
func TestGetLatestSummaryByPlatform_AcceptsLegacyWithMatchingRunMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/result/summary/4.20_External_latest.json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(makePayloadWithLegacyMetadata("regular", false))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	origURL := reportBaseURL
	setReportBaseURL(srv.URL)
	defer setReportBaseURL(origURL)

	brs := NewBaselineReportSummary()
	err := brs.GetLatestSummaryByPlatform("4.20", "External", "regular")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if brs.GetBuffer() == nil {
		t.Fatal("expected buffer to be set (legacy with matching run-mode should be accepted)")
	}
}

// TestSummaryNameIncludesWorkflow verifies that dataPath in metadata
// includes workflow when present to prevent S3 key collisions.
func TestSummaryNameIncludesWorkflow(t *testing.T) {
	tests := []struct {
		release  string
		platform string
		ts       string
		workflow string
		want     string
	}{
		{"4.20", "External", "20260101120000", "", "4.20_External_20260101120000.json"},
		{"4.20", "External", "20260101120000", "upgrade", "4.20_External_20260101120000_upgrade.json"},
		{"4.20", "None", "20260101120000", "regular", "4.20_None_20260101120000_regular.json"},
	}
	for _, tc := range tests {
		var got string
		if tc.workflow != "" {
			got = fmt.Sprintf("%s_%s_%s_%s.json", tc.release, tc.platform, tc.ts, tc.workflow)
		} else {
			got = fmt.Sprintf("%s_%s_%s.json", tc.release, tc.platform, tc.ts)
		}
		if got != tc.want {
			t.Errorf("summaryName(%s,%s,%s,%s) = %q, want %q", tc.release, tc.platform, tc.ts, tc.workflow, got, tc.want)
		}
	}
}
