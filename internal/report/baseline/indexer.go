package baseline

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudfront"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
)

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

// CreateBaselineIndex list all object from S3 Bucket, extract metadata,
// and calculate the latest by release and platform type, creating a index.json
// object.
func (brs *BaselineConfig) CreateBaselineIndex() error {
	svcS3, _, err := brs.createS3Clients()
	if err != nil {
		return fmt.Errorf("failed to create S3 client and validate bucket: %w", err)
	}

	// List all the objects in the bucket and create index.
	objects, err := ListObjects(svcS3, brs.bucketRegion, brs.bucketName, "api/v0/result/summary/")
	if err != nil {
		return err
	}

	index := baselineIndex{
		LastUpdate: time.Now().Format(time.RFC3339),
		Latest:     make(map[string]*baselineIndexItem),
	}
	// calculate the index for each object (summary)
	for _, obj := range objects {
		// Keys must have the following format: {ocpVersion}_{platformType}_{timestamp}.json
		objectKey := *obj.Key

		name := objectKey[strings.LastIndex(objectKey, "/")+1:]
		if name == "index.json" {
			continue
		}

		// read the object to extract metadata/tags from 'setup.api'
		objReader, err := svcS3.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(brs.bucketName),
			Key:    aws.String(objectKey),
		})
		if err != nil {
			log.Errorf("failed to get object %s: %v", objectKey, err)
			continue
		}

		defer objReader.Body.Close()
		bd := &BaselineData{}
		body, err := io.ReadAll(objReader.Body)
		if err != nil {
			log.Errorf("failed to read object data %s: %v", objectKey, err)
			continue
		}

		bd.SetRawData(body)
		tags, err := bd.GetSetupTags()
		if err != nil {
			log.Errorf("failed to deserialize tags/metadata from summary data: %v", err)
		}

		log.Infof("Processing summary object: %s", name)
		log.Infof("Processing metadata: %v", tags)
		openShiftRelease := strings.Split(name, "_")[0]
		if _, ok := tags["openshiftRelease"]; ok {
			openShiftRelease = tags["openshiftRelease"].(string)
		} else {
			log.Warnf("missing openshiftRelease tag in metadata, extracting from name: %v", openShiftRelease)
		}

		platformType := strings.Split(name, "_")[1]
		if _, ok := tags["platformType"]; ok {
			platformType = tags["platformType"].(string)
		} else {
			log.Warnf("missing platformType tag in metadata, extracting from name: %v", platformType)
		}

		executionDate := strings.Split(name, "_")[2]
		if _, ok := tags["executionDate"]; ok {
			executionDate = tags["executionDate"].(string)
		} else {
			log.Warnf("missing executionDate tag in metadata, extracting from name: %v", executionDate)
		}

		// Creating summary item for baseline result
		res := &baselineIndexItem{
			Date:             executionDate,
			Name:             strings.Split(name, ".json")[0],
			Path:             objectKey,
			Size:             fmt.Sprintf("%d", *obj.Size),
			OpenShiftRelease: openShiftRelease,
			PlatformType:     platformType,
			Tags:             tags,
		}
		// spew.Dump(res)
		index.Results = append(index.Results, res)
		latestIndexKey := fmt.Sprintf("%s_%s", openShiftRelease, platformType)
		existing, ok := index.Latest[latestIndexKey]
		if !ok {
			res.IsLatest = true
			index.Latest[latestIndexKey] = res
		} else {
			if existing.Date < res.Date {
				existing.IsLatest = false
				res.IsLatest = true
				index.Latest[latestIndexKey] = res
			}
		}
	}

	// Copy latest to respective path under /<version>_<platform>_latest.json
	for kLatest, latest := range index.Latest {
		latestObjectKey := fmt.Sprintf("api/v0/result/summary/%s_latest.json", kLatest)
		log.Infof("Creating latest object for %q to %q", kLatest, latestObjectKey)
		_, err := svcS3.CopyObject(&s3.CopyObjectInput{
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
	_, err = svcS3.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(brs.bucketName),
		Key:    aws.String(indexObjectKey),
		Body:   strings.NewReader(string(indexJSON)),
	})
	if err != nil {
		return fmt.Errorf("failed to upload index to bucket: %w", err)
	}

	// Expire cache from cloudfront distribution
	svcCloudfront, err := createCloudFrontClient(brs.bucketRegion)
	if err != nil {
		return fmt.Errorf("failed to create cloudfront client: %w", err)
	}
	invalidationPathsStr := []string{
		"/result/summary/index.json",
		"/result/summary/*_latest.json",
	}
	log.Infof("Creating cache invalidation for %v", strings.Join(invalidationPathsStr, " "))
	var invalidationPaths []*string
	for _, path := range invalidationPathsStr {
		invalidationPaths = append(invalidationPaths, aws.String(path))
	}
	_, err = svcCloudfront.CreateInvalidation(&cloudfront.CreateInvalidationInput{
		DistributionId: aws.String(brs.cloudfrontDistributionID),
		InvalidationBatch: &cloudfront.InvalidationBatch{
			CallerReference: aws.String(time.Now().Format(time.RFC3339)),
			Paths: &cloudfront.Paths{
				Quantity: aws.Int64(int64(len(invalidationPaths))),
				Items:    invalidationPaths,
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

// ListObjects lists all the objects in the bucket.
func ListObjects(svc *s3.S3, bucketRegion, bucketName, path string) ([]*s3.Object, error) {
	input := &s3.ListObjectsInput{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(path),
	}
	resp, err := svc.ListObjects(input)
	if err != nil {
		return nil, err
	}
	return resp.Contents, nil
}
