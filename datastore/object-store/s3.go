package objectstore

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	log "github.com/sirupsen/logrus"
)

type S3Client struct {
	session *session.Session
	opts    ObjectStoreOptions
}

func NewS3Client(opts ObjectStoreOptions) (ObjectStore, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(opts.Region),
		Credentials: credentials.NewStaticCredentials(opts.AccessKey, opts.SecretKey, opts.SessionToken),
	})
	if err != nil {
		return nil, err
	}

	client := &S3Client{
		session: sess,
		opts:    opts,
	}

	return client, nil

}

func (s3 *S3Client) Save(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		log.WithError(err).Errorf("Unable to open file %q, %v", filename, err)
		return err
	}

	defer file.Close()

	uploader := s3manager.NewUploader(s3.session)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s3.opts.Bucket),
		Key:    aws.String(filename),
		Body:   file,
	})
	if err != nil {
		log.WithError(err).Errorf("Unable to upload %q to %q, %v", filename, s3.opts.Bucket, err)
		return err
	}

	log.Printf("Successfully uploaded %q to %q\n", filename, s3.opts.Bucket)
	return nil
}
