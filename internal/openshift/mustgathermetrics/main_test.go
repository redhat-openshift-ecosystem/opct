package mustgathermetrics

import (
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"
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
