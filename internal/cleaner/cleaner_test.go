package cleaner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createTestTarGz(files map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, err
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			return nil, err
		}
	}

	if err := tarWriter.Close(); err != nil {
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func TestScanPatchTarGzipReaderFor(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string
		expectedFiles map[string]string
		expectError   bool
	}{
		{
			name: "patch JSON file",
			files: map[string]string{
				"resources/cluster/machineconfiguration.openshift.io_v1_controllerconfigs.json": `{"items":[{"spec":{"internalRegistryPullSecret":"SECRET"}}]}`,
			},
			expectedFiles: map[string]string{
				"resources/cluster/machineconfiguration.openshift.io_v1_controllerconfigs.json": `{"items":[{"spec":{"internalRegistryPullSecret":"REDACTED"}}]}`,
			},
			expectError: false,
		},
		{
			name: "no patch needed",
			files: map[string]string{
				"some/other/file.txt": "This is a test file.",
			},
			expectedFiles: map[string]string{
				"some/other/file.txt": "This is a test file.",
			},
			expectError: false,
		},
		// {
		// 	name: "invalid gzip",
		// 	files: map[string]string{
		// 		"invalid.gz": "This is not a valid gzip file.",
		// 	},
		// 	expectedFiles: nil,
		// 	expectError:   true,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tarGzData, err := createTestTarGz(tt.files)
			assert.NoError(t, err)

			reader := bytes.NewReader(tarGzData)
			resp, size, err := ScanPatchTarGzipReaderFor(reader)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Greater(t, size, 0)

			// Read the response tar.gz
			gzipReader, err := gzip.NewReader(resp)
			assert.NoError(t, err)
			defer gzipReader.Close()

			tarReader := tar.NewReader(gzipReader)
			for {
				header, err := tarReader.Next()
				if err == io.EOF {
					break
				}
				assert.NoError(t, err)

				expectedContent, ok := tt.expectedFiles[header.Name]
				assert.True(t, ok)

				content, err := io.ReadAll(tarReader)
				assert.NoError(t, err)
				assert.Equal(t, expectedContent, string(content))
			}
		})
	}
}

func TestApplyJSONPatch(t *testing.T) {
	tests := []struct {
		name        string
		filepath    string
		data        []byte
		expected    []byte
		expectError bool
	}{
		{
			name:     "valid patch",
			filepath: "resources/cluster/machineconfiguration.openshift.io_v1_controllerconfigs.json",
			data:     []byte(`{"items":[{"spec":{"internalRegistryPullSecret":"SECRET"}}]}`),
			expected: []byte(`{"items":[{"spec":{"internalRegistryPullSecret":"REDACTED"}}]}`),
		},
		{
			name:        "invalid patch",
			filepath:    "resources/cluster/machineconfiguration.openshift.io_v1_controllerconfigs.json",
			data:        []byte(`invalid json`),
			expected:    nil,
			expectError: true,
		},
		{
			name:        "no patch available",
			filepath:    "nonexistent/file.json",
			data:        []byte(`{"key":"value"}`),
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := applyJSONPatch(tt.filepath, tt.data)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
