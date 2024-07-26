package exp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/sonobuoy/pkg/errlog"
)

type submitInput struct {
	archive      string
	bucketName   string
	bucketRegion string
	objectKey    string
}

var argsPublish submitInput
var cmdPublish = &cobra.Command{
	Use:   "publish archive.tar.gz",
	Short: "(Experimental) Publish resultss.",
	Long:  "Experimental command to publis results to OPCT services. This is a experimental feature used on CI only, dont use it. :)",
	Run:   cmdPublishRun,
	Args:  cobra.ExactArgs(1),
}

func init() {
	cmdPublish.Flags().StringVarP(
		&argsPublish.objectKey, "key", "k", "",
		"Object key to use when uploading the archive to the bucket, when not set the uploads/ path will be prepended to the filename.",
	)
}

func cmdPublishRun(cmd *cobra.Command, args []string) {
	argsPublish.archive = args[0]
	if err := publishResult(&argsPublish); err != nil {
		errlog.LogError(errors.Wrapf(err, "could not publish results: %v", args[0]))
		os.Exit(1)
	}

}

// createS3Client creates an S3 client with the specified region
func createS3Client(region string) (*s3.S3, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, err
	}

	svc := s3.New(sess)
	return svc, nil
}

// checkRequiredParams checks if the required env to enable feature is set, then
// set the default storage names for experimental feature.
func checkRequiredParams(input *submitInput) bool {

	if os.Getenv("OPCT_ENABLE_EXP_PUBLISH") == "" {
		return false
	}

	input.bucketName = "openshift-provider-certification"
	input.bucketRegion = "us-west-2"

	return true
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

// processResult reads the artifacts and show it as an report format.
func publishResult(input *submitInput) error {

	log.Info("Publishing the results to storage...")

	if !checkRequiredParams(input) {
		return fmt.Errorf("missing required parameters or dependencies to enable this feature. Please wait for stable release to use it")
	}

	// create s3 client
	svc, err := createS3Client(input.bucketRegion)
	if err != nil {
		return err
	}
	// Check if the bucket exists
	bucketExists, err := checkBucketExists(svc, input.bucketName)
	if err != nil {
		return err
	}

	if !bucketExists {
		return fmt.Errorf("the OPCT storage does not exists")
	}

	// Upload the archive to the bucket
	file, err := os.Open(input.archive)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", input.archive, err)
	}
	defer file.Close()

	// objects key, when specified, must end with the filename
	filename := filepath.Base(input.archive)
	objectKey := fmt.Sprintf("uploads/%s", filename)
	if input.objectKey != "" {
		if !strings.HasSuffix(input.objectKey, filename) {
			return fmt.Errorf("object key must end with the archive name")
		}
		objectKey = input.objectKey
	}
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(input.bucketName),
		Key:    aws.String(objectKey),
		Body:   file,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to upload file %s to bucket %s", filename, input.bucketName)
	}
	log.Info("Results published successfully to s3://", input.bucketName, "/", objectKey)

	return nil
}
