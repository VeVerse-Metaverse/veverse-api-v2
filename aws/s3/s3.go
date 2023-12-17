package s3

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gofrs/uuid"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var S3 *s3.S3
var Session *session.Session

var awsRegion = os.Getenv("AWS_S3_REGION")
var awsBucketName = os.Getenv("AWS_S3_BUCKET")

func Setup() (err error) {
	if awsRegion == "" || awsBucketName == "" {
		log.Fatalf("required s3 env not provided")
	}

	config := aws.NewConfig()
	config.Region = aws.String(awsRegion)
	Session, err = session.NewSession(config)
	if err != nil {
		log.Fatalf("failed to initialize a new AWS session: %v", err)
	}

	S3 = s3.New(Session)
	if S3 == nil {
		return fmt.Errorf("failed to create a S3 client")
	}

	return nil
}

func GetS3KeyForEntityFile(entityId uuid.UUID, fileId uuid.UUID) string {
	return fmt.Sprintf("%s/%s", entityId.String(), fileId.String())
}

func GetS3KeyForEntityUrl(url string) string {
	return filepath.Base(filepath.Dir(url)) + "/" + filepath.Base(url)
}

func GetS3UrlForEntityFile(entityId uuid.UUID, fileId uuid.UUID) string {
	return fmt.Sprintf("https://%s.s3-%s.amazonaws.com/%s/%s", awsBucketName, awsRegion, entityId.String(), fileId.String())
}

func GetS3UrlForFile(key string) string {
	return fmt.Sprintf("https://%s.s3-%s.amazonaws.com/%s", awsBucketName, awsRegion, key)
}

func GetS3PresignedDownloadUrlForEntityFile(key string, duration time.Duration) (string, error) {
	params := &s3.GetObjectInput{
		Bucket: aws.String(awsBucketName),
		Key:    aws.String(key),
	}

	if duration == 0 {
		duration = 360 * time.Minute
	}

	req, _ := S3.GetObjectRequest(params)
	url, err := req.Presign(duration) // Set link expiration time
	if err != nil {
		return "", fmt.Errorf("failed to get presigned url: %s", err.Error())
	}

	return url, err
}

func GetS3PresignedUploadUrlForEntityFile(key string, duration time.Duration) (string, error) {
	params := &s3.PutObjectInput{
		Bucket: aws.String(awsBucketName),
		Key:    aws.String(key),
	}

	if duration == 0 {
		duration = 360 * time.Minute
	}

	req, _ := S3.PutObjectRequest(params)
	url, err := req.Presign(duration) // Set link expiration time
	if err != nil {
		return "", fmt.Errorf("failed to get presigned url: %s", err.Error())
	}

	return url, err
}

func UploadObject(key string, body io.Reader, mime string, public bool, metadata *map[string]string, tags *map[string]string) (err error) {
	if mime == "" {
		mime = "application/octet-stream" // Default MIME
	}

	acl := "private"
	if public {
		acl = "public-read"
	}

	m := map[string]*string{}
	if metadata != nil {
		for k, v := range *metadata {
			m[k] = aws.String(v)
		}
	}

	tagging := ""
	if tags != nil {
		for k, v := range *tags {
			if tagging == "" {
				tagging = fmt.Sprintf("%s=%s", k, v)
			} else {
				tagging = fmt.Sprintf("%s\n%s=%s", tagging, k, v)
			}
		}
	}

	var filename string
	if m["originalPath"] != nil {
		filename = filepath.Base(*m["originalPath"])
	}

	uploader := s3manager.NewUploader(Session)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket:             aws.String(awsBucketName),
		Key:                aws.String(key),
		Body:               body,
		Metadata:           m,
		ContentType:        aws.String(mime),
		ContentDisposition: aws.String(fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filename))),
		ACL:                aws.String(acl),
		Tagging:            aws.String(tagging),
	})

	return err
}

func ObjectExists(key string) bool {
	_, err := S3.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(awsBucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchKey:
				return false
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and Message from an error.
			fmt.Println(err.Error())
		}
		return false
	}

	return true
}

func DeleteObject(key string) error {
	_, err := S3.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(awsBucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		return err
	}

	return S3.WaitUntilObjectNotExists(&s3.HeadObjectInput{
		Bucket: aws.String(awsBucketName),
		Key:    aws.String(key),
	})
}
