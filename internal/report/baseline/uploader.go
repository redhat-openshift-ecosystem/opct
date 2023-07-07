package baseline

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	log "github.com/sirupsen/logrus"
)

func (brs *BaselineConfig) UploadBaseline(filePath, resPath string, meta map[string]string, dryRun bool) error {
	svcS3, uploader, err := brs.createS3Clients()
	if err != nil {
		return fmt.Errorf("failed to create S3 client and validate bucket: %w", err)
	}

	// Upload the archive to the bucket
	log.Debugf("UploadBaseline(): opening file %s", filePath)
	fdArchive, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer fdArchive.Close()

	// Object names and paths
	filenameArtifact := filepath.Base(filePath)
	objectKeyArtifact := fmt.Sprintf("uploads/%s", filenameArtifact)
	filenameSummary := resPath + "/opct-report-summary.json"
	objectKeySummary := "api/v0/result/summary/" + meta["dataPath"]

	meta["objectArtifact"] = objectKeyArtifact
	meta["objectSummary"] = objectKeySummary

	// when metadata is set, parse it and add it to the object
	// upload artifact to bucket
	log.Debugf("UploadBaseline(): uploading to object %s", objectKeyArtifact)
	s3ObjectURI := "s3://" + brs.bucketName + "/" + objectKeyArtifact
	if !dryRun {
		_, err := uploader.Upload(&s3manager.UploadInput{
			Bucket:   aws.String(brs.bucketName),
			Key:      aws.String(objectKeyArtifact),
			Metadata: aws.StringMap(meta),
			Body:     fdArchive,
		})
		if err != nil {
			return fmt.Errorf("failed to upload file %s to bucket %s: %w", filenameArtifact, brs.bucketName, err)
		}
		log.Info("Results published successfully to ", s3ObjectURI)
		// log.Debugf("UploadBaseline(): putObjectOutput: %v", putOutArchive)
	} else {
		log.Warnf("DRY-RUN mode: skipping upload to %s", s3ObjectURI)
	}

	// Saving summary to the bucket

	log.Debugf("UploadBaseline(): opening file %q", filenameSummary)
	fdSummary, err := os.Open(filenameSummary)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filenameSummary, err)
	}
	defer fdArchive.Close()

	log.Debugf("UploadBaseline(): uploading baseline summary to %q", objectKeySummary)
	s3ObjectURI = "s3://" + brs.bucketName + "/" + objectKeySummary
	if !dryRun {
		_, err = svcS3.PutObject(&s3.PutObjectInput{
			Bucket:   aws.String(brs.bucketName),
			Key:      aws.String(objectKeySummary),
			Body:     fdSummary,
			Metadata: aws.StringMap(meta),
		})
		if err != nil {
			return fmt.Errorf("failed to upload file %s to bucket %s: %w", filenameSummary, brs.bucketName, err)
		}
		log.Info("Results published successfully to s3://", brs.bucketName, "/", objectKeySummary)

	} else {
		log.Warnf("DRY-RUN mode: skipping upload to %s", s3ObjectURI)
	}

	return nil
}
