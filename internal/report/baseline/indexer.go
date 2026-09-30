package baseline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	log "github.com/sirupsen/logrus"
)

// s3API is the subset of the S3 client used by the indexer, letting tests
// substitute a mock. SDK v2 ships no generated interface equivalent to v1's
// s3iface.S3API, so consumers declare the methods they depend on.
type s3API interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	CopyObject(context.Context, *s3.CopyObjectInput, ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
}

type baselineIndexItem struct {
	Date             string                 `json:"date"`
	Name             string                 `json:"name"`
	Path             string                 `json:"path"`
	OpenShiftRelease string                 `json:"openshift_version"`
	Provider         string                 `json:"provider"`
	PlatformType     string                 `json:"platform_type"`
	Status           string                 `json:"status"`
	Size             string                 `json:"size"`
	IsLatest         bool                   `json:"is_latest"`
	Tags             map[string]interface{} `json:"tags"`
}
type baselineIndex struct {
	LastUpdate string                        `json:"date"`
	Status     string                        `json:"status"`
	Results    []*baselineIndexItem          `json:"results"`
	Latest     map[string]*baselineIndexItem `json:"latest"`
}

// CreateBaselineIndex lists objects from S3, extracts metadata,
// and calculates the latest by release and platform type, creating an index.json.
// It uses incremental indexing: loads the existing index and only fetches
// metadata for new objects not already in the index.
func (brs *BaselineConfig) CreateBaselineIndex(ctx context.Context) error {
	svcS3, _, err := brs.createS3Clients(ctx)
	if err != nil {
		return fmt.Errorf("failed to create S3 client and validate bucket: %w", err)
	}

	objects, err := ListObjects(ctx, svcS3, brs.bucketRegion, brs.bucketName, "api/v0/result/summary/")
	if err != nil {
		return err
	}

	// Load existing index from S3 for incremental updates.
	existingIndex, err := brs.loadIndexFromS3(ctx, svcS3)
	if err != nil {
		log.Warnf("Could not load existing index, performing full reindex: %v", err)
	}

	// Build set of paths currently in S3 to prune deleted objects from the index.
	currentPaths := make(map[string]struct{}, len(objects))
	for _, obj := range objects {
		currentPaths[aws.ToString(obj.Key)] = struct{}{}
	}

	knownPaths := make(map[string]bool)

	index := baselineIndex{
		LastUpdate: time.Now().Format(time.RFC3339),
		Latest:     make(map[string]*baselineIndexItem),
	}

	// Carry over existing results that still exist in S3.
	if existingIndex != nil {
		for _, item := range existingIndex.Results {
			if _, ok := currentPaths[item.Path]; !ok {
				continue
			}
			item.IsLatest = false
			index.Results = append(index.Results, item)
			knownPaths[item.Path] = true
		}
	}

	var newCount int
	for _, obj := range objects {
		objectKey := aws.ToString(obj.Key)

		name := objectKey[strings.LastIndex(objectKey, "/")+1:]
		if name == "index.json" || strings.HasSuffix(name, "_latest.json") {
			continue
		}

		if knownPaths[objectKey] {
			continue
		}

		newCount++
		item, err := brs.fetchObjectMetadata(ctx, svcS3, objectKey, name, obj)
		if err != nil {
			log.Errorf("failed to process object %s: %v", objectKey, err)
			continue
		}
		index.Results = append(index.Results, item)
	}

	log.Infof("Index update: %d existing, %d new, %d total", len(knownPaths), newCount, len(index.Results))

	// Recalculate latest for all results.
	for _, res := range index.Results {
		res.IsLatest = false
		latestIndexKey := fmt.Sprintf("%s_%s", res.OpenShiftRelease, res.PlatformType)
		existing, ok := index.Latest[latestIndexKey]
		if !ok {
			res.IsLatest = true
			index.Latest[latestIndexKey] = res
		} else if existing.Date < res.Date {
			existing.IsLatest = false
			res.IsLatest = true
			index.Latest[latestIndexKey] = res
		}
	}

	// Copy latest to respective path under /<version>_<platform>_latest.json
	for kLatest, latest := range index.Latest {
		latestObjectKey := fmt.Sprintf("api/v0/result/summary/%s_latest.json", kLatest)
		log.Infof("Creating latest object for %q to %q", kLatest, latestObjectKey)
		_, err := svcS3.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(brs.bucketName),
			CopySource: aws.String(fmt.Sprintf("%v/%v", brs.bucketName, latest.Path)),
			Key:        aws.String(latestObjectKey),
		})
		if err != nil {
			log.Errorf("Couldn't create latest object %s: %v", kLatest, err)
		}
	}

	// Save the new index to the bucket.
	indexJSON, err := json.Marshal(index)
	if err != nil {
		return fmt.Errorf("unable to save index to json: %w", err)
	}

	// Save the index to the bucket
	_, err = svcS3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(brs.bucketName),
		Key:    aws.String(indexObjectKey),
		Body:   strings.NewReader(string(indexJSON)),
	})
	if err != nil {
		return fmt.Errorf("failed to upload index to bucket: %w", err)
	}

	// Expire cache from cloudfront distribution
	svcCloudfront, err := createCloudFrontClient(ctx, brs.bucketRegion)
	if err != nil {
		return fmt.Errorf("failed to create cloudfront client: %w", err)
	}
	invalidationPathsStr := []string{
		"/result/summary/index.json",
		"/result/summary/*_latest.json",
	}
	log.Infof("Creating cache invalidation for %v", strings.Join(invalidationPathsStr, " "))
	_, err = svcCloudfront.CreateInvalidation(ctx, &cloudfront.CreateInvalidationInput{
		DistributionId: aws.String(brs.cloudfrontDistributionID),
		InvalidationBatch: &cftypes.InvalidationBatch{
			CallerReference: aws.String(time.Now().Format(time.RFC3339)),
			Paths: &cftypes.Paths{
				Quantity: aws.Int32(int32(len(invalidationPathsStr))),
				Items:    invalidationPathsStr,
			},
		},
	})
	if err != nil {

		log.Warnf("failed to create cache invalidation: %v", err)
		fmt.Printf(`Index updated. Run the following command to invalidate index.cache:
aws cloudfront create-invalidation \
	--distribution-id %s \
	--paths %s`, brs.cloudfrontDistributionID, strings.Join(invalidationPathsStr, " "))
		fmt.Println()
	}
	return nil
}

// loadIndexFromS3 reads the existing index.json from S3 for incremental updates.
func (brs *BaselineConfig) loadIndexFromS3(ctx context.Context, svc s3API) (*baselineIndex, error) {
	resp, err := svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(brs.bucketName),
		Key:    aws.String(indexObjectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get index object: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Warnf("failed to close response body: %v", cerr)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read index body: %w", err)
	}

	var idx baselineIndex
	if err := json.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("failed to parse index JSON: %w", err)
	}

	// Filter out any _latest entries that may have polluted a previous index.
	var cleaned []*baselineIndexItem
	for _, item := range idx.Results {
		if !strings.HasSuffix(item.Name, "_latest") {
			cleaned = append(cleaned, item)
		}
	}
	idx.Results = cleaned

	return &idx, nil
}

// fetchObjectMetadata downloads a single S3 object and extracts its index metadata.
func (brs *BaselineConfig) fetchObjectMetadata(ctx context.Context, svc s3API, objectKey, name string, obj s3types.Object) (*baselineIndexItem, error) {
	objReader, err := svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(brs.bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s: %w", objectKey, err)
	}
	defer func() {
		if cerr := objReader.Body.Close(); cerr != nil {
			log.Warnf("failed to close object body: %v", cerr)
		}
	}()

	body, err := io.ReadAll(objReader.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object data %s: %w", objectKey, err)
	}

	bd := &BaselineData{}
	bd.SetRawData(body)
	tags, err := bd.GetSetupTags()
	if err != nil {
		log.Errorf("failed to deserialize tags/metadata from summary data: %v", err)
	}

	log.Infof("Processing summary object: %s", name)
	log.Debugf("Processing metadata: %v", tags)

	parts := strings.Split(name, "_")
	if len(parts) < 3 {
		return nil, fmt.Errorf("malformed object name %q: expected at least 3 underscore-separated parts", name)
	}

	openShiftRelease := parts[0]
	if v, ok := tags["openshiftRelease"].(string); ok && v != "" {
		openShiftRelease = v
	} else {
		log.Warnf("missing openshiftRelease tag in metadata, extracting from name: %v", openShiftRelease)
	}

	platformType := parts[1]
	if v, ok := tags["platformType"].(string); ok && v != "" {
		platformType = v
	} else {
		log.Warnf("missing platformType tag in metadata, extracting from name: %v", platformType)
	}

	executionDate := parts[2]
	if v, ok := tags["executionDate"].(string); ok && v != "" {
		executionDate = v
	} else {
		log.Warnf("missing executionDate tag in metadata, extracting from name: %v", executionDate)
	}

	return &baselineIndexItem{
		Date:             executionDate,
		Name:             strings.Split(name, ".json")[0],
		Path:             objectKey,
		Size:             fmt.Sprintf("%d", aws.ToInt64(obj.Size)),
		OpenShiftRelease: openShiftRelease,
		PlatformType:     platformType,
		Tags:             tags,
	}, nil
}

// ListObjects lists all objects in the bucket, paginating through all results.
func ListObjects(ctx context.Context, svc s3API, bucketRegion, bucketName, path string) ([]s3types.Object, error) {
	var objects []s3types.Object
	paginator := s3.NewListObjectsV2Paginator(svc, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(path),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		objects = append(objects, page.Contents...)
	}
	return objects, nil
}
