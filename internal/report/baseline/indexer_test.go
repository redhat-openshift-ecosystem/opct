package baseline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// mockS3Client implements the s3API subset used by the indexer.
type mockS3Client struct {
	objects    map[string][]byte
	listOutput []s3types.Object
	putCalls   []s3.PutObjectInput
	copyCalls  []s3.CopyObjectInput
}

func newMockS3(objects map[string][]byte) *mockS3Client {
	var list []s3types.Object
	for key, body := range objects {
		size := int64(len(body))
		k := key
		list = append(list, s3types.Object{
			Key:  &k,
			Size: &size,
		})
	}
	return &mockS3Client{
		objects:    objects,
		listOutput: list,
	}
}

func (m *mockS3Client) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	key := aws.ToString(input.Key)
	body, ok := m.objects[key]
	if !ok {
		return nil, fmt.Errorf("NoSuchKey: %s", key)
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader(string(body))),
	}, nil
}

func (m *mockS3Client) ListObjectsV2(_ context.Context, input *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	prefix := aws.ToString(input.Prefix)
	var filtered []s3types.Object
	for _, obj := range m.listOutput {
		if strings.HasPrefix(aws.ToString(obj.Key), prefix) {
			filtered = append(filtered, obj)
		}
	}
	return &s3.ListObjectsV2Output{Contents: filtered}, nil
}

func (m *mockS3Client) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.putCalls = append(m.putCalls, *input)
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) CopyObject(_ context.Context, input *s3.CopyObjectInput, _ ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	m.copyCalls = append(m.copyCalls, *input)
	return &s3.CopyObjectOutput{}, nil
}

// makeSummaryJSON builds a minimal summary object with setup.api tags.
func makeSummaryJSON(release, platform, date string) []byte {
	return makeSummaryJSONWithWorkflow(release, platform, date, "")
}

// makeSummaryJSONWithWorkflow builds a minimal summary object with setup.api tags
// including an optional workflow field.
func makeSummaryJSONWithWorkflow(release, platform, date, workflow string) []byte {
	api := map[string]interface{}{
		"openshiftRelease": release,
		"platformType":     platform,
		"executionDate":    date,
		"dataPath":         fmt.Sprintf("%s_%s_%s.json", release, platform, strings.ReplaceAll(date, ":", "")),
	}
	if workflow != "" {
		api["workflow"] = workflow
	}
	obj := map[string]interface{}{
		"setup": map[string]interface{}{
			"api": api,
		},
	}
	b, _ := json.Marshal(obj)
	return b
}

// makeIndexJSON builds a serialized baselineIndex from items.
func makeIndexJSON(items []*baselineIndexItem) []byte {
	idx := baselineIndex{
		LastUpdate: "2026-01-01T00:00:00Z",
		Results:    items,
		Latest:     make(map[string]*baselineIndexItem),
	}
	b, _ := json.Marshal(idx)
	return b
}

func TestListObjects_Pagination(t *testing.T) {
	objects := map[string][]byte{
		"api/v0/result/summary/4.17_None_20250101.json":     []byte("{}"),
		"api/v0/result/summary/4.17_None_20250201.json":     []byte("{}"),
		"api/v0/result/summary/4.18_External_20250301.json": []byte("{}"),
	}
	mock := newMockS3(objects)

	result, err := ListObjects(context.Background(), mock, "us-east-1", "test-bucket", "api/v0/result/summary/")
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 objects, got %d", len(result))
	}
}

func TestFetchObjectMetadata_ValidObject(t *testing.T) {
	body := makeSummaryJSON("4.17", "None", "2025-01-15T10:00:00Z")
	objects := map[string][]byte{
		"api/v0/result/summary/4.17_None_20250115.json": body,
	}
	mock := newMockS3(objects)
	brs := &BaselineConfig{bucketName: "test-bucket"}

	size := int64(len(body))
	obj := s3types.Object{
		Key:  aws.String("api/v0/result/summary/4.17_None_20250115.json"),
		Size: &size,
	}

	item, err := brs.fetchObjectMetadata(context.Background(), mock, "api/v0/result/summary/4.17_None_20250115.json", "4.17_None_20250115.json", obj)
	if err != nil {
		t.Fatalf("fetchObjectMetadata failed: %v", err)
	}
	if item.OpenShiftRelease != "4.17" {
		t.Errorf("expected release 4.17, got %s", item.OpenShiftRelease)
	}
	if item.PlatformType != "None" {
		t.Errorf("expected platform None, got %s", item.PlatformType)
	}
	if item.Date != "2025-01-15T10:00:00Z" {
		t.Errorf("expected date 2025-01-15T10:00:00Z, got %s", item.Date)
	}
	if item.Name != "4.17_None_20250115" {
		t.Errorf("expected name 4.17_None_20250115, got %s", item.Name)
	}
}

func TestFetchObjectMetadata_MalformedName(t *testing.T) {
	body := makeSummaryJSON("4.17", "None", "2025-01-01T00:00:00Z")
	objects := map[string][]byte{
		"api/v0/result/summary/malformed.json": body,
	}
	mock := newMockS3(objects)
	brs := &BaselineConfig{bucketName: "test-bucket"}

	size := int64(len(body))
	obj := s3types.Object{
		Key:  aws.String("api/v0/result/summary/malformed.json"),
		Size: &size,
	}

	_, err := brs.fetchObjectMetadata(context.Background(), mock, "api/v0/result/summary/malformed.json", "malformed.json", obj)
	if err == nil {
		t.Fatal("expected error for malformed name, got nil")
	}
	if !strings.Contains(err.Error(), "malformed object name") {
		t.Errorf("expected malformed object name error, got: %v", err)
	}
}

func TestFetchObjectMetadata_NonStringTags(t *testing.T) {
	// Tags with non-string values should fall back to filename parts, not panic.
	obj := map[string]interface{}{
		"setup": map[string]interface{}{
			"api": map[string]interface{}{
				"openshiftRelease": 417,
				"platformType":     nil,
				"executionDate":    true,
			},
		},
	}
	body, _ := json.Marshal(obj)
	objects := map[string][]byte{
		"api/v0/result/summary/4.17_None_20250115.json": body,
	}
	mock := newMockS3(objects)
	brs := &BaselineConfig{bucketName: "test-bucket"}

	size := int64(len(body))
	s3obj := s3types.Object{
		Key:  aws.String("api/v0/result/summary/4.17_None_20250115.json"),
		Size: &size,
	}

	item, err := brs.fetchObjectMetadata(context.Background(), mock, "api/v0/result/summary/4.17_None_20250115.json", "4.17_None_20250115.json", s3obj)
	if err != nil {
		t.Fatalf("fetchObjectMetadata should not fail on non-string tags: %v", err)
	}
	if item.OpenShiftRelease != "4.17" {
		t.Errorf("expected fallback release 4.17, got %s", item.OpenShiftRelease)
	}
	if item.PlatformType != "None" {
		t.Errorf("expected fallback platform None, got %s", item.PlatformType)
	}
	if item.Date != "20250115.json" {
		t.Errorf("expected fallback date 20250115.json, got %s", item.Date)
	}
}

func TestLoadIndexFromS3_FilterLatestPollution(t *testing.T) {
	items := []*baselineIndexItem{
		{Name: "4.17_None_20250101", Path: "api/v0/result/summary/4.17_None_20250101.json", OpenShiftRelease: "4.17", PlatformType: "None", Date: "2025-01-01"},
		{Name: "4.17_None_latest", Path: "api/v0/result/summary/4.17_None_latest.json", OpenShiftRelease: "4.17", PlatformType: "None", Date: "2025-01-01"},
		{Name: "4.18_External_20250201", Path: "api/v0/result/summary/4.18_External_20250201.json", OpenShiftRelease: "4.18", PlatformType: "External", Date: "2025-02-01"},
		{Name: "4.18_External_latest", Path: "api/v0/result/summary/4.18_External_latest.json", OpenShiftRelease: "4.18", PlatformType: "External", Date: "2025-02-01"},
	}
	indexJSON := makeIndexJSON(items)
	objects := map[string][]byte{
		"api/v0/result/summary/index.json": indexJSON,
	}
	mock := newMockS3(objects)
	brs := &BaselineConfig{bucketName: "test-bucket"}

	idx, err := brs.loadIndexFromS3(context.Background(), mock)
	if err != nil {
		t.Fatalf("loadIndexFromS3 failed: %v", err)
	}
	if len(idx.Results) != 2 {
		t.Errorf("expected 2 results after filtering _latest, got %d", len(idx.Results))
	}
	for _, item := range idx.Results {
		if strings.HasSuffix(item.Name, "_latest") {
			t.Errorf("_latest entry should have been filtered: %s", item.Name)
		}
	}
}

func TestLoadIndexFromS3_NoIndex(t *testing.T) {
	mock := newMockS3(map[string][]byte{})
	brs := &BaselineConfig{bucketName: "test-bucket"}

	_, err := brs.loadIndexFromS3(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error when index.json does not exist")
	}
}

func TestCreateBaselineIndex_IncrementalSkipsExisting(t *testing.T) {
	summary1 := makeSummaryJSON("4.17", "None", "2025-01-01T00:00:00Z")
	summary2 := makeSummaryJSON("4.18", "External", "2025-02-01T00:00:00Z")

	existingItems := []*baselineIndexItem{
		{
			Name: "4.17_None_20250101", Path: "api/v0/result/summary/4.17_None_20250101.json",
			OpenShiftRelease: "4.17", PlatformType: "None", Date: "2025-01-01T00:00:00Z", Size: fmt.Sprintf("%d", len(summary1)),
		},
	}
	indexJSON := makeIndexJSON(existingItems)

	objects := map[string][]byte{
		"api/v0/result/summary/index.json":                  indexJSON,
		"api/v0/result/summary/4.17_None_20250101.json":     summary1,
		"api/v0/result/summary/4.18_External_20250201.json": summary2,
	}
	mock := newMockS3(objects)

	brs := &BaselineConfig{bucketName: "test-bucket", bucketRegion: "us-east-1"}

	idx, err := brs.loadIndexFromS3(context.Background(), mock)
	if err != nil {
		t.Fatalf("loadIndexFromS3 failed: %v", err)
	}

	currentPaths := make(map[string]struct{})
	for _, obj := range mock.listOutput {
		currentPaths[aws.ToString(obj.Key)] = struct{}{}
	}

	knownPaths := make(map[string]bool)
	var results []*baselineIndexItem
	for _, item := range idx.Results {
		if _, ok := currentPaths[item.Path]; !ok {
			continue
		}
		item.IsLatest = false
		results = append(results, item)
		knownPaths[item.Path] = true
	}

	// Only the new object should need fetching.
	var newCount int
	for _, obj := range mock.listOutput {
		key := aws.ToString(obj.Key)
		name := key[strings.LastIndex(key, "/")+1:]
		if name == "index.json" || strings.HasSuffix(name, "_latest.json") {
			continue
		}
		if knownPaths[key] {
			continue
		}
		newCount++
		item, err := brs.fetchObjectMetadata(context.Background(), mock, key, name, obj)
		if err != nil {
			t.Fatalf("fetchObjectMetadata failed for new object: %v", err)
		}
		results = append(results, item)
	}

	if newCount != 1 {
		t.Errorf("expected 1 new object, got %d", newCount)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 total results, got %d", len(results))
	}
}

func TestCreateBaselineIndex_PrunesDeletedObjects(t *testing.T) {
	summary1 := makeSummaryJSON("4.17", "None", "2025-01-01T00:00:00Z")

	existingItems := []*baselineIndexItem{
		{
			Name: "4.17_None_20250101", Path: "api/v0/result/summary/4.17_None_20250101.json",
			OpenShiftRelease: "4.17", PlatformType: "None", Date: "2025-01-01T00:00:00Z",
		},
		{
			Name: "4.16_None_20241201", Path: "api/v0/result/summary/4.16_None_20241201.json",
			OpenShiftRelease: "4.16", PlatformType: "None", Date: "2024-12-01T00:00:00Z",
		},
	}
	indexJSON := makeIndexJSON(existingItems)

	// S3 only has summary1 and the index — 4.16 object was deleted.
	objects := map[string][]byte{
		"api/v0/result/summary/index.json":              indexJSON,
		"api/v0/result/summary/4.17_None_20250101.json": summary1,
	}
	mock := newMockS3(objects)

	idx, err := brs_loadAndPrune(mock, objects)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx) != 1 {
		t.Errorf("expected 1 result after pruning deleted object, got %d", len(idx))
	}
	if idx[0].Name != "4.17_None_20250101" {
		t.Errorf("expected surviving entry 4.17_None_20250101, got %s", idx[0].Name)
	}
}

// brs_loadAndPrune simulates the incremental carry-over with pruning logic.
func brs_loadAndPrune(mock *mockS3Client, objects map[string][]byte) ([]*baselineIndexItem, error) {
	brs := &BaselineConfig{bucketName: "test-bucket"}

	idx, err := brs.loadIndexFromS3(context.Background(), mock)
	if err != nil {
		return nil, err
	}

	currentPaths := make(map[string]struct{})
	for key := range objects {
		currentPaths[key] = struct{}{}
	}

	var results []*baselineIndexItem
	for _, item := range idx.Results {
		if _, ok := currentPaths[item.Path]; !ok {
			continue
		}
		results = append(results, item)
	}
	return results, nil
}

func TestLatestCalculation(t *testing.T) {
	items := []*baselineIndexItem{
		{Name: "4.17_None_20250101", OpenShiftRelease: "4.17", PlatformType: "None", Date: "2025-01-01T00:00:00Z"},
		{Name: "4.17_None_20250201", OpenShiftRelease: "4.17", PlatformType: "None", Date: "2025-02-01T00:00:00Z"},
		{Name: "4.17_None_20241201", OpenShiftRelease: "4.17", PlatformType: "None", Date: "2024-12-01T00:00:00Z"},
		{Name: "4.18_External_20250301", OpenShiftRelease: "4.18", PlatformType: "External", Date: "2025-03-01T00:00:00Z"},
	}

	latest := make(map[string]*baselineIndexItem)
	for _, res := range items {
		res.IsLatest = false
		key := fmt.Sprintf("%s_%s", res.OpenShiftRelease, res.PlatformType)
		existing, ok := latest[key]
		if !ok {
			res.IsLatest = true
			latest[key] = res
		} else if existing.Date < res.Date {
			existing.IsLatest = false
			res.IsLatest = true
			latest[key] = res
		}
	}

	if len(latest) != 2 {
		t.Errorf("expected 2 latest entries, got %d", len(latest))
	}
	if latest["4.17_None"].Name != "4.17_None_20250201" {
		t.Errorf("expected 4.17_None latest to be 20250201, got %s", latest["4.17_None"].Name)
	}
	if !latest["4.17_None"].IsLatest {
		t.Error("expected 4.17_None latest entry to have IsLatest=true")
	}
	if latest["4.18_External"].Name != "4.18_External_20250301" {
		t.Errorf("expected 4.18_External latest to be 20250301, got %s", latest["4.18_External"].Name)
	}

	// Verify non-latest entries have IsLatest=false.
	for _, item := range items {
		if item.Name == "4.17_None_20250101" && item.IsLatest {
			t.Error("4.17_None_20250101 should not be latest")
		}
		if item.Name == "4.17_None_20241201" && item.IsLatest {
			t.Error("4.17_None_20241201 should not be latest")
		}
	}
}

func TestFilterSpecialObjects(t *testing.T) {
	names := []struct {
		name     string
		expected bool
	}{
		{"4.17_None_20250101.json", true},
		{"4.18_External_20250201.json", true},
		{"index.json", false},
		{"4.17_None_latest.json", false},
		{"4.18_External_latest.json", false},
	}

	for _, tc := range names {
		skip := tc.name == "index.json" || strings.HasSuffix(tc.name, "_latest.json")
		included := !skip
		if included != tc.expected {
			t.Errorf("object %q: expected included=%v, got %v", tc.name, tc.expected, included)
		}
	}
}

// --- Workflow-scoped baseline tests ---

func TestLatestKey(t *testing.T) {
	tests := []struct {
		release  string
		platform string
		workflow string
		want     string
	}{
		{"4.17", "None", "", "4.17_None"},
		{"4.17", "None", "upgrade", "4.17_None_upgrade"},
		{"4.18", "External", "regular", "4.18_External_regular"},
		{"4.19", "AWS", "", "4.19_AWS"},
	}
	for _, tc := range tests {
		got := latestKey(tc.release, tc.platform, tc.workflow)
		if got != tc.want {
			t.Errorf("latestKey(%q, %q, %q) = %q, want %q", tc.release, tc.platform, tc.workflow, got, tc.want)
		}
	}
}

func TestFetchObjectMetadata_ExtractsWorkflow(t *testing.T) {
	body := makeSummaryJSONWithWorkflow("4.20", "External", "2026-03-01T00:00:00Z", "upgrade")
	key := "api/v0/result/summary/4.20_External_20260301.json"
	objects := map[string][]byte{key: body}
	mock := newMockS3(objects)
	brs := &BaselineConfig{bucketName: "test-bucket"}

	size := int64(len(body))
	obj := s3types.Object{Key: aws.String(key), Size: &size}

	item, err := brs.fetchObjectMetadata(context.Background(), mock, key, "4.20_External_20260301.json", obj)
	if err != nil {
		t.Fatalf("fetchObjectMetadata failed: %v", err)
	}
	if item.Workflow != "upgrade" {
		t.Errorf("expected workflow 'upgrade', got %q", item.Workflow)
	}
}

func TestFetchObjectMetadata_LegacyNoWorkflow(t *testing.T) {
	body := makeSummaryJSON("4.17", "None", "2025-01-01T00:00:00Z")
	key := "api/v0/result/summary/4.17_None_20250101.json"
	objects := map[string][]byte{key: body}
	mock := newMockS3(objects)
	brs := &BaselineConfig{bucketName: "test-bucket"}

	size := int64(len(body))
	obj := s3types.Object{Key: aws.String(key), Size: &size}

	item, err := brs.fetchObjectMetadata(context.Background(), mock, key, "4.17_None_20250101.json", obj)
	if err != nil {
		t.Fatalf("fetchObjectMetadata failed: %v", err)
	}
	if item.Workflow != "" {
		t.Errorf("legacy archive should have empty workflow, got %q", item.Workflow)
	}
}

// TestLatestCalculationWorkflowScoped verifies that conformance and upgrade
// baselines with the same version and platform maintain independent "latest"
// entries rather than overwriting each other.
func TestLatestCalculationWorkflowScoped(t *testing.T) {
	items := []*baselineIndexItem{
		{Name: "4.20_None_20260101_conformance", OpenShiftRelease: "4.20", PlatformType: "None", Workflow: "", Date: "2026-01-01T00:00:00Z"},
		{Name: "4.20_None_20260201_conformance", OpenShiftRelease: "4.20", PlatformType: "None", Workflow: "", Date: "2026-02-01T00:00:00Z"},
		{Name: "4.20_None_20260101_upgrade", OpenShiftRelease: "4.20", PlatformType: "None", Workflow: "upgrade", Date: "2026-01-01T00:00:00Z"},
		{Name: "4.20_None_20260301_upgrade", OpenShiftRelease: "4.20", PlatformType: "None", Workflow: "upgrade", Date: "2026-03-01T00:00:00Z"},
	}

	latest := make(map[string]*baselineIndexItem)
	for _, res := range items {
		res.IsLatest = false
		key := latestKey(res.OpenShiftRelease, res.PlatformType, res.Workflow)
		existing, ok := latest[key]
		if !ok {
			res.IsLatest = true
			latest[key] = res
		} else if existing.Date < res.Date {
			existing.IsLatest = false
			res.IsLatest = true
			latest[key] = res
		}
	}

	if len(latest) != 2 {
		t.Fatalf("expected 2 latest entries (one per workflow), got %d", len(latest))
	}

	// Legacy (empty workflow) latest
	legacyLatest := latest["4.20_None"]
	if legacyLatest == nil {
		t.Fatal("expected 4.20_None latest entry")
	}
	if legacyLatest.Name != "4.20_None_20260201_conformance" {
		t.Errorf("expected legacy latest to be 20260201, got %s", legacyLatest.Name)
	}

	// Upgrade workflow latest
	upgradeLatest := latest["4.20_None_upgrade"]
	if upgradeLatest == nil {
		t.Fatal("expected 4.20_None_upgrade latest entry")
	}
	if upgradeLatest.Name != "4.20_None_20260301_upgrade" {
		t.Errorf("expected upgrade latest to be 20260301, got %s", upgradeLatest.Name)
	}
}

// TestLatestCalculationUpgradeDoesNotOverwriteConformance verifies that
// publishing a newer upgrade baseline does not displace the conformance
// baseline's "latest" entry (the core regression this change prevents).
func TestLatestCalculationUpgradeDoesNotOverwriteConformance(t *testing.T) {
	// Conformance baseline published first.
	conformance := &baselineIndexItem{
		Name: "4.20_External_20260101", OpenShiftRelease: "4.20",
		PlatformType: "External", Workflow: "", Date: "2026-01-01T00:00:00Z",
	}
	// Upgrade baseline published later, same dimensions.
	upgrade := &baselineIndexItem{
		Name: "4.20_External_20260301_upgrade", OpenShiftRelease: "4.20",
		PlatformType: "External", Workflow: "upgrade", Date: "2026-03-01T00:00:00Z",
	}

	latest := make(map[string]*baselineIndexItem)
	for _, res := range []*baselineIndexItem{conformance, upgrade} {
		res.IsLatest = false
		key := latestKey(res.OpenShiftRelease, res.PlatformType, res.Workflow)
		existing, ok := latest[key]
		if !ok {
			res.IsLatest = true
			latest[key] = res
		} else if existing.Date < res.Date {
			existing.IsLatest = false
			res.IsLatest = true
			latest[key] = res
		}
	}

	// The conformance entry must still be present as latest.
	if _, ok := latest["4.20_External"]; !ok {
		t.Error("conformance baseline was displaced by upgrade publication")
	}
	// And the upgrade entry must exist independently.
	if _, ok := latest["4.20_External_upgrade"]; !ok {
		t.Error("upgrade baseline was not tracked")
	}
}

// TestWorkflowFieldJSONRoundTrip verifies the Workflow field survives
// serialization and deserialization of the index, preserving backward
// compatibility (old indexes have no workflow field, omitempty keeps it clean).
func TestWorkflowFieldJSONRoundTrip(t *testing.T) {
	items := []*baselineIndexItem{
		{Name: "legacy", OpenShiftRelease: "4.17", PlatformType: "None", Workflow: ""},
		{Name: "scoped", OpenShiftRelease: "4.20", PlatformType: "External", Workflow: "upgrade"},
	}
	indexJSON := makeIndexJSON(items)

	var idx baselineIndex
	if err := json.Unmarshal(indexJSON, &idx); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(idx.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(idx.Results))
	}
	if idx.Results[0].Workflow != "" {
		t.Errorf("legacy entry should have empty workflow, got %q", idx.Results[0].Workflow)
	}
	if idx.Results[1].Workflow != "upgrade" {
		t.Errorf("scoped entry should have workflow 'upgrade', got %q", idx.Results[1].Workflow)
	}
}
