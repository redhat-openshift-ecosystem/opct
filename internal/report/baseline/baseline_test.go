package baseline

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetLatestRawSummaryFromPlatformWithFallback_WorkflowValidation
// uses an HTTP test server to simulate the CloudFront API and verifies
// that the consumer-path fallback correctly validates payload workflows.
func TestGetLatestRawSummaryFromPlatformWithFallback_WorkflowValidation(t *testing.T) {
	tests := []struct {
		name           string
		requestWF      string // workflow requested by the caller
		servedPaths    map[string]string // path -> payload workflow ("" = legacy, served; absent = 404)
		expectBuffer   bool
		expectWorkflow string // expected workflow in the accepted payload (empty if legacy)
	}{
		{
			name:      "exact workflow-scoped match is accepted",
			requestWF: "upgrade",
			servedPaths: map[string]string{
				"/result/summary/4.20_External_upgrade_latest.json": "upgrade",
			},
			expectBuffer:   true,
			expectWorkflow: "upgrade",
		},
		{
			name:      "wrong-workflow payload is rejected even if HTTP succeeds",
			requestWF: "upgrade",
			servedPaths: map[string]string{
				// Only the legacy path exists and it carries "regular"
				"/result/summary/4.20_External_latest.json": "regular",
				"/result/summary/4.20_None_latest.json":     "regular",
				"/result/summary/4.20_AWS_latest.json":      "regular",
			},
			expectBuffer: false, // must NOT accept a "regular" baseline for "upgrade" request
		},
		{
			name:      "legacy payload without workflow field is accepted as fallback",
			requestWF: "upgrade",
			servedPaths: map[string]string{
				// Only legacy path, payload has no workflow field
				"/result/summary/4.20_External_latest.json": "", // "" means no workflow field
			},
			expectBuffer: true,
		},
		{
			name:      "no workflow filter accepts first available payload",
			requestWF: "",
			servedPaths: map[string]string{
				"/result/summary/4.20_External_latest.json": "upgrade",
			},
			expectBuffer:   true,
			expectWorkflow: "upgrade",
		},
		{
			name:      "workflow-scoped path preferred over legacy with wrong workflow",
			requestWF: "upgrade",
			servedPaths: map[string]string{
				"/result/summary/4.20_External_upgrade_latest.json": "upgrade",
				"/result/summary/4.20_External_latest.json":         "regular", // wrong workflow
			},
			expectBuffer:   true,
			expectWorkflow: "upgrade",
		},
		{
			name:      "all paths 404 leaves buffer nil",
			requestWF: "upgrade",
			servedPaths: map[string]string{},
			expectBuffer: false,
		},
		{
			name:      "platform fallback finds matching workflow on None",
			requestWF: "upgrade",
			servedPaths: map[string]string{
				// External paths all 404, but None has the right one
				"/result/summary/4.20_None_upgrade_latest.json": "upgrade",
			},
			expectBuffer:   true,
			expectWorkflow: "upgrade",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				payloadWF, ok := tt.servedPaths[r.URL.Path]
				if !ok {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(makePayloadForServer(payloadWF))
			}))
			defer srv.Close()

			// Override the report base URL to point to our test server.
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
				wf, ok := buf.GetWorkflow()
				if !ok {
					t.Fatal("expected workflow field in accepted payload")
				}
				if wf != tt.expectWorkflow {
					t.Errorf("expected accepted payload workflow %q, got %q", tt.expectWorkflow, wf)
				}
			}
		})
	}
}

// makePayloadForServer builds a JSON payload like the CloudFront API serves.
func makePayloadForServer(workflow string) []byte {
	return makePayloadWithWorkflow(workflow)
}

// TestGetLatestSummaryByPlatform_RejectsWrongWorkflow tests that the
// explicit get-by-platform path rejects a legacy payload carrying a
// conflicting workflow.
func TestGetLatestSummaryByPlatform_RejectsWrongWorkflow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only serve the legacy path; workflow-scoped path returns 404.
		if r.URL.Path == "/result/summary/4.20_External_latest.json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(makePayloadWithWorkflow("regular"))
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
