package baseline

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	// awsMaxAttempts counts total attempts, while the SDK v1 setting it replaces
	// counted retries after the first attempt. 11 preserves the original 10 retries.
	awsMaxAttempts   = 11
	awsRetryMaxDelay = 30 * time.Second
)

// newAWSConfig loads a shared AWS configuration with retry and backoff settings.
// Retry is configured explicitly rather than left to the SDK default of 3
// attempts, preserving the tolerance the v1 code established for bulk indexing
// and large uploads. Doing so takes precedence over AWS_MAX_ATTEMPTS and
// AWS_RETRY_MODE. The v1 throttle delays have no v2 equivalent; the standard
// retryer derives throttle backoff from MaxBackoff.
func newAWSConfig(ctx context.Context, region string) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithRetryer(func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) {
				o.MaxAttempts = awsMaxAttempts
				o.MaxBackoff = awsRetryMaxDelay
			})
		}),
	)
}

// createS3Client creates an S3 client and uploader with the specified region.
// manager is deprecated in favour of feature/s3/transfermanager, which is still
// pre-1.0 and free to break its API between minor releases. Staying on manager
// until that module reaches v1; tracked separately from this SDK migration.
//
//lint:ignore SA1019 transfermanager replacement is not yet v1.
func createS3Client(ctx context.Context, region string) (*s3.Client, *manager.Uploader, error) {
	cfg, err := newAWSConfig(ctx, region)
	if err != nil {
		return nil, nil, err
	}

	svc := s3.NewFromConfig(cfg)
	//lint:ignore SA1019 transfermanager replacement is not yet v1.
	uploader := manager.NewUploader(svc)

	return svc, uploader, nil
}

// createCloudFrontClient creates a CloudFront client with the specified region.
func createCloudFrontClient(ctx context.Context, region string) (*cloudfront.Client, error) {
	cfg, err := newAWSConfig(ctx, region)
	if err != nil {
		return nil, err
	}

	return cloudfront.NewFromConfig(cfg), nil
}

// checkBucketExists checks if the bucket exists in the S3 storage.
func checkBucketExists(ctx context.Context, svc *s3.Client, bucket string) (bool, error) {
	_, err := svc.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return false, fmt.Errorf("failed to check if bucket exists: %v", err)
	}
	return true, nil
}
