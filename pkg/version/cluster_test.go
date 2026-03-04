package version

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestClusterVersion_Fields(t *testing.T) {
	// Test ClusterVersion struct field assignment
	cv := ClusterVersion{
		OpenShift:  "4.15.0",
		Kubernetes: "v1.28.0",
	}

	if cv.OpenShift != "4.15.0" {
		t.Errorf("ClusterVersion.OpenShift = %q, want %q", cv.OpenShift, "4.15.0")
	}

	if cv.Kubernetes != "v1.28.0" {
		t.Errorf("ClusterVersion.Kubernetes = %q, want %q", cv.Kubernetes, "v1.28.0")
	}
}

func TestGetClusterVersion_NoKubeconfig(t *testing.T) {
	// Save original viper state
	originalKubeconfig := viper.Get("kubeconfig")
	defer func() {
		if originalKubeconfig != nil {
			viper.Set("kubeconfig", originalKubeconfig)
		} else {
			// Reset viper to clean state
			viper.Reset()
		}
	}()

	// Ensure kubeconfig is not set
	viper.Reset()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	result, err := GetClusterVersion()

	// Restore stdout
	if closeErr := w.Close(); closeErr != nil {
		t.Fatalf("Failed to close pipe writer: %v", closeErr)
	}
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, r); copyErr != nil {
		t.Fatalf("Failed to copy pipe output: %v", copyErr)
	}
	output := buf.String()

	// Verify error is returned
	if err == nil {
		t.Error("GetClusterVersion() expected error when KUBECONFIG is not set, got nil")
	}

	expectedError := "KUBECONFIG is not set"
	if err.Error() != expectedError {
		t.Errorf("GetClusterVersion() error = %q, want %q", err.Error(), expectedError)
	}

	// Verify result is nil
	if result != nil {
		t.Errorf("GetClusterVersion() result = %v, want nil", result)
	}

	// Verify output contains expected message
	expectedOutputs := []string{
		"Cluster version:",
		"unknown (KUBECONFIG is not set)",
	}

	for _, expected := range expectedOutputs {
		if !strings.Contains(output, expected) {
			t.Errorf("GetClusterVersion() output missing %q\nGot: %s", expected, output)
		}
	}
}

func TestGetClusterVersion_WithKubeconfigSet(t *testing.T) {
	// This test verifies behavior when KUBECONFIG is set but connection fails
	// Note: Full integration testing would require mocking the Kubernetes client,
	// which is not straightforward with the current implementation due to:
	// 1. Direct os.Exit() calls that terminate the test process
	// 2. Tight coupling with client creation
	// 3. No dependency injection for client interfaces
	//
	// For comprehensive testing, the GetClusterVersion function would need refactoring to:
	// - Accept client interfaces as dependencies
	// - Return errors instead of calling os.Exit()
	// - Separate output formatting from business logic

	t.Skip("Skipping integration test - requires refactoring to support dependency injection")

	// Future improvement: Refactor GetClusterVersion to accept interfaces:
	// func GetClusterVersion(configClient ConfigClientInterface, kubeClient KubeClientInterface) (*ClusterVersion, error)
}

func TestClusterVersion_ZeroValue(t *testing.T) {
	// Test zero value initialization
	var cv ClusterVersion

	if cv.OpenShift != "" {
		t.Errorf("Zero value ClusterVersion.OpenShift = %q, want empty string", cv.OpenShift)
	}

	if cv.Kubernetes != "" {
		t.Errorf("Zero value ClusterVersion.Kubernetes = %q, want empty string", cv.Kubernetes)
	}
}

func TestClusterVersion_Equality(t *testing.T) {
	cv1 := ClusterVersion{
		OpenShift:  "4.15.0",
		Kubernetes: "v1.28.0",
	}

	cv2 := ClusterVersion{
		OpenShift:  "4.15.0",
		Kubernetes: "v1.28.0",
	}

	cv3 := ClusterVersion{
		OpenShift:  "4.16.0",
		Kubernetes: "v1.29.0",
	}

	// Test equality
	if cv1 != cv2 {
		t.Errorf("Expected cv1 == cv2, but they are not equal")
	}

	// Test inequality
	if cv1 == cv3 {
		t.Errorf("Expected cv1 != cv3, but they are equal")
	}
}

// Note: Additional test coverage would require refactoring GetClusterVersion to:
// 1. Remove direct os.Exit() calls - return errors instead
// 2. Accept client interfaces as parameters (dependency injection)
// 3. Separate output formatting (fmt.Printf) from business logic
//
// Example refactored signature:
//   func GetClusterVersion(ctx context.Context, client ClientInterface) (*ClusterVersion, error)
//
// This would enable:
// - Mocking client responses
// - Testing error paths without process termination
// - Testing business logic independently of I/O
// - Improved testability and maintainability
