package mustgathermetrics

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"

	"github.com/ulikunitz/xz"
)

func TestReadDecompressedMetricWithinLimit(t *testing.T) {
	payload := strings.Repeat("a", 1024)
	gzReader := gzipReader(t, payload)

	got, err := readDecompressedMetric(gzReader, "test.json.gz", 2048)
	if err != nil {
		t.Fatalf("readDecompressedMetric() error = %v", err)
	}
	if string(got) != payload {
		t.Fatalf("readDecompressedMetric() got %d bytes, want %d", len(got), len(payload))
	}
}

func TestReadDecompressedMetricRejectsOversized(t *testing.T) {
	payload := strings.Repeat("a", 2048)
	gzReader := gzipReader(t, payload)

	_, err := readDecompressedMetric(gzReader, "test.json.gz", 1024)
	if err == nil {
		t.Fatal("readDecompressedMetric() expected error for oversized payload")
	}
	if !strings.Contains(err.Error(), "exceeds maximum decompressed size") {
		t.Fatalf("readDecompressedMetric() error = %v, want size limit error", err)
	}
}

func TestExtractMetricsRejectsOversizedNonMetricMember(t *testing.T) {
	const archiveLimit int64 = 4096
	largePayload := make([]byte, archiveLimit*2)

	archive := buildTestTarXZ(t, map[string][]byte{
		"other/large.bin": largePayload,
	})

	tarReader, err := readArchiveWithLimit(archive, archiveLimit)
	if err != nil {
		t.Fatalf("readArchiveWithLimit() error = %v", err)
	}

	mg := &MustGatherMetrics{charts: map[string]*Chart{}}
	err = mg.extractMetrics(tarReader)
	if err == nil {
		t.Fatal("extractMetrics() expected error for oversized non-metric member")
	}
	if !strings.Contains(err.Error(), "archive exceeds maximum decompressed size") {
		t.Fatalf("extractMetrics() error = %v, want archive size limit error", err)
	}
}

func buildTestTarXZ(t *testing.T, entries map[string][]byte) *bytes.Buffer {
	t.Helper()

	var xzBuf bytes.Buffer
	xzWriter, err := xz.NewWriter(&xzBuf)
	if err != nil {
		t.Fatalf("xz.NewWriter: %v", err)
	}

	tarWriter := tar.NewWriter(xzWriter)
	for name, content := range entries {
		header := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("tar.WriteHeader(%q): %v", name, err)
		}
		if _, err := tarWriter.Write(content); err != nil {
			t.Fatalf("tar.Write(%q): %v", name, err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tar.Close: %v", err)
	}
	if err := xzWriter.Close(); err != nil {
		t.Fatalf("xz.Close: %v", err)
	}

	return &xzBuf
}

func gzipReader(t *testing.T, payload string) io.Reader {
	t.Helper()

	compressed := gzipCompress(t, payload)
	gzReader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	t.Cleanup(func() { _ = gzReader.Close() })
	return gzReader
}

func gzipCompress(t *testing.T, payload string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := io.WriteString(gz, payload); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}
