package cleaner

import (
	"bytes"
	"testing"
)

func TestScanContentForLeaks_OpenShiftToken(t *testing.T) {
	content := []byte(`some log line
token: sha256~abcdefghijklmnopqrstuvwxyz01234567890ABCDEF
another line`)
	findings := ScanContentForLeaks("test.log", content)
	if len(findings) == 0 {
		t.Fatal("expected to find OpenShift User Token")
	}
	if findings[0].Pattern != "OpenShift User Token" {
		t.Errorf("expected pattern 'OpenShift User Token', got '%s'", findings[0].Pattern)
	}
	if findings[0].Line != 2 {
		t.Errorf("expected line 2, got %d", findings[0].Line)
	}
}

func TestScanContentForLeaks_AWSKey(t *testing.T) {
	content := []byte(`config:
  accessKeyId: AKIAIOSFODNN7EXAMPLE
  secretKey: something`)
	findings := ScanContentForLeaks("config.yaml", content)
	found := false
	for _, f := range findings {
		if f.Pattern == "AWS IAM Unique Identifier" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find AWS IAM Unique Identifier")
	}
}

func TestScanContentForLeaks_PrivateKey(t *testing.T) {
	content := []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF9PbnGy0AHB7MhgHcTz6sE2I2yPB
anotherlineofbase64anotherlineofbase64anotherlineofbase64anotherlineofba
-----END RSA PRIVATE KEY-----`)
	findings := ScanContentForLeaks("key.pem", content)
	if len(findings) == 0 {
		t.Fatal("expected to find Private Key (PEM)")
	}
	if findings[0].Pattern != "Private Key (PEM)" {
		t.Errorf("expected pattern 'Private Key (PEM)', got '%s'", findings[0].Pattern)
	}
}

func TestScanContentForLeaks_GenericSecret(t *testing.T) {
	content := []byte(`DATABASE_PASSWORD=supersecretvalue123`)
	findings := ScanContentForLeaks("env.txt", content)
	found := false
	for _, f := range findings {
		if f.Pattern == "Generic Secret (key=value unquoted)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find Generic Secret (key=value unquoted)")
	}
}

func TestScanContentForLeaks_GitHubToken(t *testing.T) {
	content := []byte(`token = "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh12"`)
	findings := ScanContentForLeaks("config.json", content)
	found := false
	for _, f := range findings {
		if f.Pattern == "GitHub Personal Access Token" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find GitHub Personal Access Token")
	}
}

func TestScanContentForLeaks_NoFindings(t *testing.T) {
	content := []byte(`this is a normal log file
with no secrets or sensitive data
just regular cluster information
openshift version 4.22`)
	findings := ScanContentForLeaks("normal.log", content)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestScanContentForLeaks_BinarySkip(t *testing.T) {
	content := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	findings := ScanContentForLeaks("image.png", content)
	if len(findings) != 0 {
		t.Errorf("expected no findings for binary file, got %d", len(findings))
	}
}

func TestScanContentForLeaks_EmptyContent(t *testing.T) {
	findings := ScanContentForLeaks("empty.txt", []byte{})
	if len(findings) != 0 {
		t.Errorf("expected no findings for empty content, got %d", len(findings))
	}
}

func TestScanContentForLeaks_GCPKey(t *testing.T) {
	content := []byte(`api_key: AIzaSyA1234567890abcdefghijklmnopqrstuv`)
	findings := ScanContentForLeaks("gcp.yaml", content)
	found := false
	for _, f := range findings {
		if f.Pattern == "GCP API Key" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find GCP API Key")
	}
}

func TestScanContentForLeaks_ContainerRegistryAuth(t *testing.T) {
	// Pattern matches auths JSON embedded inside a k8s resource string field
	content := []byte(`"internalRegistryPullSecret":"{\"auths\":{\"quay.io\":{\"auth\":\"dGVzdHVzZXI6dGVzdHBhc3N3b3JkMTIzNA==\"}}}"`)
	findings := ScanContentForLeaks("config.json", content)
	found := false
	for _, f := range findings {
		if f.Pattern == "Container Registry Authentication" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find Container Registry Authentication")
	}
}

func TestScanContentForLeaks_KubernetesJWT(t *testing.T) {
	// JWT with base64-encoded "sub":"system:serviceaccount: in the payload
	content := []byte(`token: eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50Om9wY3Q6ZGVmYXVsdCJ9.signature_value_here`)
	findings := ScanContentForLeaks("sa-token.txt", content)
	found := false
	for _, f := range findings {
		if f.Pattern == "Kubernetes Service Account JWT" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find Kubernetes Service Account JWT")
	}
}

func TestScanContentForLeaks_AzureADSecret(t *testing.T) {
	content := []byte(`AZURE_CLIENT_SECRET=abc8Q~abcdefghijklmnopqrstuvwxyz1234567`)
	findings := ScanContentForLeaks("azure.env", content)
	found := false
	for _, f := range findings {
		if f.Pattern == "Azure AD Client Secret" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find Azure AD Client Secret")
	}
}

// Redaction Tests

func TestScanAndRedactLeaks_OpenShiftToken(t *testing.T) {
	original := []byte(`some log line
token: sha256~abcdefghijklmnopqrstuvwxyz01234567890ABCDEF
another line`)
	redacted, findings := ScanAndRedactLeaks("test.log", original)

	if len(findings) == 0 {
		t.Fatal("expected to find OpenShift User Token")
	}

	// Verify redaction marker is present
	if !bytes.Contains(redacted, []byte(redactionMarker)) {
		t.Error("expected redacted content to contain <REDACTED_BY_OPCT>")
	}

	// Verify original secret is NOT present
	if bytes.Contains(redacted, []byte("sha256~abcdefghijklmnopqrstuvwxyz01234567890ABCDEF")) {
		t.Error("original token still present in redacted content")
	}

	// Verify surrounding context is preserved
	if !bytes.Contains(redacted, []byte("some log line")) {
		t.Error("surrounding context was removed")
	}
	if !bytes.Contains(redacted, []byte("another line")) {
		t.Error("surrounding context was removed")
	}
}

func TestScanAndRedactLeaks_AWSKey(t *testing.T) {
	original := []byte(`config:
  accessKeyId: AKIAIOSFODNN7EXAMPLE
  secretKey: something`)
	redacted, findings := ScanAndRedactLeaks("config.yaml", original)

	if len(findings) == 0 {
		t.Fatal("expected to find AWS IAM Unique Identifier")
	}

	// Verify AWS key is redacted
	if bytes.Contains(redacted, []byte("AKIAIOSFODNN7EXAMPLE")) {
		t.Error("original AWS key still present in redacted content")
	}

	if !bytes.Contains(redacted, []byte(redactionMarker)) {
		t.Error("expected redacted content to contain <REDACTED_BY_OPCT>")
	}
}

func TestScanAndRedactLeaks_PrivateKey(t *testing.T) {
	original := []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyF9PbnGy0AHB7MhgHcTz6sE2I2yPB
anotherlineofbase64anotherlineofbase64anotherlineofbase64anotherlineofba
-----END RSA PRIVATE KEY-----`)
	redacted, findings := ScanAndRedactLeaks("key.pem", original)

	if len(findings) == 0 {
		t.Fatal("expected to find Private Key (PEM)")
	}

	// Verify private key block is redacted
	if bytes.Contains(redacted, []byte("MIIEpAIBAAKCAQEA")) {
		t.Error("original private key still present in redacted content")
	}

	if !bytes.Contains(redacted, []byte(redactionMarker)) {
		t.Error("expected redacted content to contain <REDACTED_BY_OPCT>")
	}
}

func TestScanAndRedactLeaks_MultipleSecretsOnSameLine(t *testing.T) {
	original := []byte(`AWS_KEY=AKIAIOSFODNN7EXAMPLE and GITHUB_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh12`)
	redacted, findings := ScanAndRedactLeaks("env.txt", original)

	// Should detect both secrets
	if len(findings) < 2 {
		t.Errorf("expected to find at least 2 secrets, got %d", len(findings))
	}

	// Both should be redacted
	if bytes.Contains(redacted, []byte("AKIAIOSFODNN7EXAMPLE")) {
		t.Error("AWS key still present in redacted content")
	}
	if bytes.Contains(redacted, []byte("ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh12")) {
		t.Error("GitHub token still present in redacted content")
	}

	// Should have two redaction markers
	count := bytes.Count(redacted, []byte(redactionMarker))
	if count < 2 {
		t.Errorf("expected at least 2 redaction markers, got %d", count)
	}
}

func TestScanAndRedactLeaks_NoSecretsNoChange(t *testing.T) {
	original := []byte(`this is a normal log file
with no secrets or sensitive data
just regular cluster information
openshift version 4.22`)
	redacted, findings := ScanAndRedactLeaks("normal.log", original)

	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}

	// Content should be unchanged
	if !bytes.Equal(original, redacted) {
		t.Error("clean content was modified")
	}
}

func TestScanAndRedactLeaks_BinarySkip(t *testing.T) {
	original := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	redacted, findings := ScanAndRedactLeaks("image.png", original)

	if len(findings) != 0 {
		t.Errorf("expected no findings for binary file, got %d", len(findings))
	}

	// Binary content should be unchanged
	if !bytes.Equal(original, redacted) {
		t.Error("binary content was modified")
	}
}

func TestScanAndRedactLeaks_EmptyContent(t *testing.T) {
	original := []byte{}
	redacted, findings := ScanAndRedactLeaks("empty.txt", original)

	if len(findings) != 0 {
		t.Errorf("expected no findings for empty content, got %d", len(findings))
	}

	if len(redacted) != 0 {
		t.Error("empty content was modified")
	}
}

func TestScanAndRedactLeaks_PreservesLineStructure(t *testing.T) {
	original := []byte("line 1\nline 2 with AWS_KEY=AKIAIOSFODNN7EXAMPLE here\nline 3\nline 4\n")
	// Save original for comparison
	originalCopy := make([]byte, len(original))
	copy(originalCopy, original)

	redacted, findings := ScanAndRedactLeaks("test.log", original)

	if len(findings) == 0 {
		t.Fatal("expected to find AWS key")
	}

	// Count lines (should be same before and after)
	originalLines := bytes.Count(originalCopy, []byte("\n"))
	redactedLines := bytes.Count(redacted, []byte("\n"))

	if originalLines != redactedLines {
		t.Errorf("line count changed: original=%d, redacted=%d\nOriginal:\n%q\nRedacted:\n%q",
			originalLines, redactedLines, string(originalCopy), string(redacted))
	}

	// Verify AWS key is redacted
	if bytes.Contains(redacted, []byte("AKIAIOSFODNN7EXAMPLE")) {
		t.Error("AWS key still present")
	}

	// Verify structure
	if !bytes.Contains(redacted, []byte("line 1")) {
		t.Error("line 1 was removed")
	}
	if !bytes.Contains(redacted, []byte("line 2 with AWS_KEY=")) {
		t.Error("line 2 prefix was removed")
	}
	if !bytes.Contains(redacted, []byte("line 3")) {
		t.Error("line 3 was removed")
	}
	if !bytes.Contains(redacted, []byte("line 4")) {
		t.Error("line 4 was removed")
	}
}

func BenchmarkScanContentForLeaks(b *testing.B) {
	content := make([]byte, 100*1024)
	for i := range content {
		content[i] = byte('a' + (i % 26))
		if i%80 == 79 {
			content[i] = '\n'
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ScanContentForLeaks("benchmark.log", content)
	}
}

func BenchmarkScanAndRedactLeaks(b *testing.B) {
	content := make([]byte, 100*1024)
	for i := range content {
		content[i] = byte('a' + (i % 26))
		if i%80 == 79 {
			content[i] = '\n'
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ScanAndRedactLeaks("benchmark.log", content)
	}
}
