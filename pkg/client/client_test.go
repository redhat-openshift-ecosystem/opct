package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"k8s.io/client-go/rest"
)

func TestNewClient(t *testing.T) {
	// Note: NewClient() requires a functional Kubernetes cluster to fully succeed
	// because it creates the Sonobuoy client which makes API calls.
	// These tests focus on error conditions that occur before cluster communication.

	tests := []struct {
		name          string
		setupEnv      func()
		cleanupEnv    func()
		wantErr       bool
		errorContains string
	}{
		{
			name: "error when no kubeconfig is set",
			setupEnv: func() {
				_ = os.Unsetenv("KUBECONFIG")
				viper.Reset()
			},
			cleanupEnv: func() {},
			wantErr:    true,
			errorContains: "--kubeconfig or KUBECONFIG environment variable must be set",
		},
		{
			name: "error when kubeconfig file does not exist",
			setupEnv: func() {
				_ = os.Setenv("KUBECONFIG", "/nonexistent/kubeconfig")
			},
			cleanupEnv: func() {
				_ = os.Unsetenv("KUBECONFIG")
			},
			wantErr:       true,
			errorContains: "no such file or directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer tt.cleanupEnv()

			cli, err := NewClient()

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewClient() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("NewClient() error = %v, want error containing %v", err, tt.errorContains)
				}
				return
			}

			if err != nil {
				t.Errorf("NewClient() unexpected error = %v", err)
				return
			}

			if cli == nil {
				t.Error("NewClient() returned nil client")
				return
			}

			if cli.KClient == nil {
				t.Error("NewClient() KClient is nil")
			}

			if cli.SClient == nil {
				t.Error("NewClient() SClient is nil")
			}

			if cli.RestConfig == nil {
				t.Error("NewClient() RestConfig is nil")
			}
		})
	}
}

func TestCreateRestConfig(t *testing.T) {
	// Create a temporary kubeconfig file for testing
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")

	kubeconfigContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfigContent), 0644); err != nil {
		t.Fatalf("Failed to create test kubeconfig: %v", err)
	}

	tests := []struct {
		name          string
		setupEnv      func()
		cleanupEnv    func()
		wantErr       bool
		errorContains string
		validateConfig func(*testing.T, *rest.Config)
	}{
		{
			name: "successful config creation with KUBECONFIG env var",
			setupEnv: func() {
				_ = os.Setenv("KUBECONFIG", kubeconfigPath)
			},
			cleanupEnv: func() {
				_ = os.Unsetenv("KUBECONFIG")
			},
			wantErr: false,
			validateConfig: func(t *testing.T, cfg *rest.Config) {
				if cfg.Host != "https://localhost:6443" {
					t.Errorf("Expected host https://localhost:6443, got %s", cfg.Host)
				}
			},
		},
		{
			name: "successful config creation with viper kubeconfig",
			setupEnv: func() {
				_ = os.Unsetenv("KUBECONFIG")
				viper.Set("kubeconfig", kubeconfigPath)
			},
			cleanupEnv: func() {
				viper.Reset()
			},
			wantErr: false,
			validateConfig: func(t *testing.T, cfg *rest.Config) {
				if cfg.Host != "https://localhost:6443" {
					t.Errorf("Expected host https://localhost:6443, got %s", cfg.Host)
				}
			},
		},
		{
			name: "error when KUBECONFIG env var is not set and viper is empty",
			setupEnv: func() {
				_ = os.Unsetenv("KUBECONFIG")
				viper.Reset()
			},
			cleanupEnv: func() {},
			wantErr:    true,
			errorContains: "--kubeconfig or KUBECONFIG environment variable must be set",
		},
		{
			name: "error when kubeconfig file does not exist (KUBECONFIG env)",
			setupEnv: func() {
				_ = os.Setenv("KUBECONFIG", "/nonexistent/path/kubeconfig")
			},
			cleanupEnv: func() {
				_ = os.Unsetenv("KUBECONFIG")
			},
			wantErr:       true,
			errorContains: "no such file or directory",
		},
		{
			name: "error when kubeconfig file does not exist (viper)",
			setupEnv: func() {
				_ = os.Unsetenv("KUBECONFIG")
				viper.Set("kubeconfig", "/another/nonexistent/kubeconfig")
			},
			cleanupEnv: func() {
				viper.Reset()
			},
			wantErr:       true,
			errorContains: "does not exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer tt.cleanupEnv()

			cfg, err := createRestConfig()

			if tt.wantErr {
				if err == nil {
					t.Errorf("createRestConfig() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("createRestConfig() error = %v, want error containing %v", err, tt.errorContains)
				}
				return
			}

			if err != nil {
				t.Errorf("createRestConfig() unexpected error = %v", err)
				return
			}

			if cfg == nil {
				t.Error("createRestConfig() returned nil config")
				return
			}

			if tt.validateConfig != nil {
				tt.validateConfig(t, cfg)
			}
		})
	}
}

func TestCreateRestConfig_PriorityOrder(t *testing.T) {
	// Test that KUBECONFIG env var takes priority over viper config
	tmpDir := t.TempDir()

	// Create two different kubeconfig files
	envKubeconfigPath := filepath.Join(tmpDir, "env-kubeconfig")
	viperKubeconfigPath := filepath.Join(tmpDir, "viper-kubeconfig")

	envContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://env-server:6443
  name: env-cluster
contexts:
- context:
    cluster: env-cluster
    user: env-user
  name: env-context
current-context: env-context
users:
- name: env-user
  user:
    token: env-token
`
	viperContent := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://viper-server:6443
  name: viper-cluster
contexts:
- context:
    cluster: viper-cluster
    user: viper-user
  name: viper-context
current-context: viper-context
users:
- name: viper-user
  user:
    token: viper-token
`

	if err := os.WriteFile(envKubeconfigPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to create env kubeconfig: %v", err)
	}
	if err := os.WriteFile(viperKubeconfigPath, []byte(viperContent), 0644); err != nil {
		t.Fatalf("Failed to create viper kubeconfig: %v", err)
	}

	// Set both KUBECONFIG env var and viper config
	_ = os.Setenv("KUBECONFIG", envKubeconfigPath)
	viper.Set("kubeconfig", viperKubeconfigPath)
	defer func() {
		_ = os.Unsetenv("KUBECONFIG")
		viper.Reset()
	}()

	cfg, err := createRestConfig()
	if err != nil {
		t.Fatalf("createRestConfig() unexpected error = %v", err)
	}

	// Should use KUBECONFIG env var (priority)
	if cfg.Host != "https://env-server:6443" {
		t.Errorf("Expected KUBECONFIG env var to take priority, got host %s", cfg.Host)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
