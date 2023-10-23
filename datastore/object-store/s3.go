package objectstore

import (
	"os"
	"strings"

	"github.com/frain-dev/convoy/util"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/frain-dev/convoy/pkg/log"
)

type S3Client struct {
	session *session.Session
	opts    ObjectStoreOptions
}

func NewS3Client(opts ObjectStoreOptions) (ObjectStore, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(opts.Region),
		Endpoint:    aws.String(opts.Endpoint),
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

	name := filename

	if util.IsStringEmpty(s3.opts.Prefix) {
		names := strings.Split(filename, "/tmp/")
		if len(names) > 1 {
			name = names[1]
		}
	} else {
		name = strings.Replace(filename, "/tmp", s3.opts.Prefix, 1)
	}

	uploader := s3manager.NewUploader(s3.session)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s3.opts.Bucket),
		Key:    aws.String(name),
		Body:   file,
	})

	if err != nil {
		log.WithError(err).Errorf("Unable to save %q to %q, %v", filename, s3.opts.Bucket, err)
		return err
	}

	log.Printf("Successfully saved %q to %q\n", filename, s3.opts.Bucket)
	return nil
}
