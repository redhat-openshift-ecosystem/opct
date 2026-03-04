package version

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/redhat-openshift-ecosystem/opct/pkg"
	"github.com/vmware-tanzu/sonobuoy/pkg/buildinfo"
)

func TestVersionContext_String(t *testing.T) {
	tests := []struct {
		name    string
		version VersionContext
		want    string
	}{
		{
			name: "development version with commit",
			version: VersionContext{
				Name:    "test-project",
				Version: "0.0.0",
				Commit:  "abc123def",
			},
			want: "OPCT CLI: 0.0.0+abc123def",
		},
		{
			name: "release version without commit",
			version: VersionContext{
				Name:    "test-project",
				Version: "1.2.3",
				Commit:  "abc123def",
			},
			want: "OPCT CLI: 1.2.3",
		},
		{
			name: "unknown version",
			version: VersionContext{
				Name:    "test-project",
				Version: "unknown",
				Commit:  "unknown",
			},
			want: "OPCT CLI: unknown",
		},
		{
			name: "semver with v prefix",
			version: VersionContext{
				Name:    "test-project",
				Version: "v0.6.1",
				Commit:  "commit-hash",
			},
			want: "OPCT CLI: v0.6.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.version.String()
			if got != tt.want {
				t.Errorf("VersionContext.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionContext_stringPluginsImage(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	vc := VersionContext{
		Name:    "test-project",
		Version: "1.0.0",
		Commit:  "abc123",
	}

	vc.stringPluginsImage()

	// Restore stdout
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close pipe writer: %v", err)
	}
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("Failed to copy pipe output: %v", err)
	}
	output := buf.String()

	// Verify output contains expected plugin images
	expectedStrings := []string{
		"Images versions:",
		pkg.SonobuoyImage,
		buildinfo.Version,
		pkg.PluginsImage,
		pkg.CollectorImage,
		pkg.MustGatherMonitoringImage,
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("stringPluginsImage() output missing %q\nGot: %s", expected, output)
		}
	}
}

func TestNewCmdVersion(t *testing.T) {
	cmd := NewCmdVersion()

	if cmd == nil {
		t.Fatal("NewCmdVersion() returned nil")
	}

	if cmd.Use != "version" {
		t.Errorf("NewCmdVersion().Use = %q, want %q", cmd.Use, "version")
	}

	if cmd.Short == "" {
		t.Error("NewCmdVersion().Short is empty")
	}

	if cmd.Run == nil {
		t.Error("NewCmdVersion().Run is nil")
	}

	// Verify the command has expected properties
	expectedShort := "Print opct CLI version"
	if cmd.Short != expectedShort {
		t.Errorf("NewCmdVersion().Short = %q, want %q", cmd.Short, expectedShort)
	}
}

func TestVersionGlobalVariable(t *testing.T) {
	// Test that the global Version variable is properly initialized
	if Version.Name == "" {
		t.Error("Version.Name is empty")
	}

	if Version.Version == "" {
		t.Error("Version.Version is empty")
	}

	if Version.Commit == "" {
		t.Error("Version.Commit is empty")
	}

	// Verify default values
	expectedName := "openshift-provider-cert"
	if Version.Name != expectedName {
		t.Errorf("Version.Name = %q, want %q", Version.Name, expectedName)
	}
}

func TestVersionContext_Fields(t *testing.T) {
	// Test struct field marshaling
	vc := VersionContext{
		Name:    "test-name",
		Version: "test-version",
		Commit:  "test-commit",
	}

	if vc.Name != "test-name" {
		t.Errorf("VersionContext.Name = %q, want %q", vc.Name, "test-name")
	}

	if vc.Version != "test-version" {
		t.Errorf("VersionContext.Version = %q, want %q", vc.Version, "test-version")
	}

	if vc.Commit != "test-commit" {
		t.Errorf("VersionContext.Commit = %q, want %q", vc.Commit, "test-commit")
	}
}
