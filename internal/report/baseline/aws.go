package baseline

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudfront"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// createS3Client creates an S3 client with the specified region
func createS3Client(region string) (*s3.S3, *s3manager.Uploader, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, nil, err
	}

	svc := s3.New(sess)

	// upload managers 	https://docs.aws.amazon.com/sdk-for-go/api/service/s3/
	// Create an uploader with the session and default options
	uploader := s3manager.NewUploader(sess)

	return svc, uploader, nil
}

// createCloudFrontClient creates an S3 client with the specified region
func createCloudFrontClient(region string) (*cloudfront.CloudFront, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: "opct",
		Config: aws.Config{
			Region: aws.String(region),
		},
	})
	if err != nil {
		return nil, err
	}

	svc := cloudfront.New(sess)
	return svc, nil
}

// checkBucketExists checks if the bucket exists in the S3 storage.
func checkBucketExists(svc *s3.S3, bucket string) (bool, error) {
	_, err := svc.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return false, fmt.Errorf("failed to check if bucket exists: %v", err)
	}
	return true, nil
}
