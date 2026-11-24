package run

import (
	"testing"

	"github.com/redhat-openshift-ecosystem/opct/internal/opct/plugin"
)

func TestUnsupportedSkipChecksAlias(t *testing.T) {
	cmd := NewCmdRun()
	if cmd.Flags().Lookup("unsupported-skip-checks") == nil {
		t.Fatal("unsupported-skip-checks flag is not registered")
	}
	if err := cmd.Flags().Set("unsupported-skip-checks", "true"); err != nil {
		t.Fatalf("setting unsupported-skip-checks: %v", err)
	}

	got, err := cmd.Flags().GetBool("devel-skip-checks")
	if err != nil {
		t.Fatalf("getting devel-skip-checks: %v", err)
	}
	if !got {
		t.Error("unsupported-skip-checks did not set devel-skip-checks")
	}
}

func TestModeWorkflowAlias(t *testing.T) {
	cmd := NewCmdRun()
	if cmd.Flags().Lookup("mode") == nil {
		t.Fatal("mode flag is not registered")
	}
	if err := cmd.Flags().Set("mode", plugin.WorkflowDisconnected); err != nil {
		t.Fatalf("setting mode: %v", err)
	}

	got, err := cmd.Flags().GetString("workflow")
	if err != nil {
		t.Fatalf("getting workflow: %v", err)
	}
	if got != plugin.WorkflowDisconnected {
		t.Errorf("mode did not set workflow: got %q", got)
	}
}

func TestValidateWorkflow(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		mirror  string
		wantErr bool
	}{
		{name: "regular is valid", mode: plugin.WorkflowRegular},
		{name: "upgrade is valid", mode: plugin.WorkflowUpgrade},
		{
			name: "disconnected with mirror is valid",
			mode: plugin.WorkflowDisconnected,
			// resolves against the cluster; only the mirror requirement is
			// asserted here, the provider resolution warns on failure.
			mirror: "mirror.example.com/opct",
		},
		{
			name:    "disconnected without mirror is invalid",
			mode:    plugin.WorkflowDisconnected,
			wantErr: true,
		},
		{name: "unknown workflow is invalid", mode: "bogus", wantErr: true},
		{name: "empty workflow is invalid", mode: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &RunOptions{mode: tc.mode, imageRepository: tc.mirror}
			err := r.validateWorkflow()
			if (err != nil) != tc.wantErr {
				t.Errorf("validateWorkflow(mode=%q, mirror=%q) error = %v, wantErr %v",
					tc.mode, tc.mirror, err, tc.wantErr)
			}
		})
	}
}

func TestProviderJSONForPlatform(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		mode     string
		want     string
	}{
		{
			name:     "external regular keeps provider",
			platform: "External",
			mode:     plugin.WorkflowRegular,
			want:     `{"type":"external"}`,
		},
		{
			name:     "external disconnected merges flag",
			platform: "External",
			mode:     plugin.WorkflowDisconnected,
			want:     `{"Disconnected":true,"type":"external"}`,
		},
		{
			name:     "none resolves to skeleton",
			platform: "None",
			mode:     plugin.WorkflowRegular,
			want:     `{"type":"skeleton"}`,
		},
		{
			name:     "aws regular emits nothing",
			platform: "AWS",
			mode:     plugin.WorkflowRegular,
			want:     "",
		},
		{
			name:     "aws disconnected emits provider with flag",
			platform: "AWS",
			mode:     plugin.WorkflowDisconnected,
			want:     `{"Disconnected":true,"type":"aws"}`,
		},
		{
			name:     "gcp disconnected maps to gce",
			platform: "GCP",
			mode:     plugin.WorkflowDisconnected,
			want:     `{"Disconnected":true,"type":"gce"}`,
		},
		{
			name:     "baremetal disconnected maps to skeleton",
			platform: "BareMetal",
			mode:     plugin.WorkflowDisconnected,
			want:     `{"Disconnected":true,"type":"skeleton"}`,
		},
		{
			name:     "vsphere disconnected emits provider with flag",
			platform: "VSphere",
			mode:     plugin.WorkflowDisconnected,
			want:     `{"Disconnected":true,"type":"vsphere"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := providerJSONForPlatform(tc.platform, tc.mode)
			if got != tc.want {
				t.Errorf("providerJSONForPlatform(%q, %q) = %q, want %q",
					tc.platform, tc.mode, got, tc.want)
			}
		})
	}
}

func TestWithDisconnectedFlag(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		wantOutput string
	}{
		{
			name:       "external provider gains disconnected flag",
			provider:   `{"type":"external"}`,
			wantOutput: `{"Disconnected":true,"type":"external"}`,
		},
		{
			name:       "already disconnected config returned unchanged",
			provider:   `{"type":"external","Disconnected":true}`,
			wantOutput: `{"type":"external","Disconnected":true}`,
		},
		{
			name:       "skeleton provider gains disconnected flag",
			provider:   `{"type":"skeleton"}`,
			wantOutput: `{"Disconnected":true,"type":"skeleton"}`,
		},
		{
			name:       "invalid provider JSON left unchanged",
			provider:   "not-json{",
			wantOutput: "not-json{",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := withDisconnectedFlag(tc.provider)
			if got != tc.wantOutput {
				t.Errorf("withDisconnectedFlag(%q) = %q, want %q", tc.provider, got, tc.wantOutput)
			}
		})
	}
}
