package objectstore

import (
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

type S3Client struct {
	session *session.Session
	opts    ObjectStoreOptions
	logger  log.Logger
}

func NewS3Client(opts ObjectStoreOptions, logger log.Logger) (ObjectStore, error) {
	sess, err := session.NewSession(&aws.Config{
		S3ForcePathStyle: aws.Bool(true),
		Region:           aws.String(opts.Region),
		Endpoint:         aws.String(opts.Endpoint),
		Credentials:      credentials.NewStaticCredentials(opts.AccessKey, opts.SecretKey, opts.SessionToken),
	})
	if err != nil {
		return nil, err
	}

	client := &S3Client{
		session: sess,
		opts:    opts,
		logger:  logger,
	}

	return client, nil
}

func (s3 *S3Client) Save(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		s3.logger.Error(fmt.Sprintf("Unable to open file %q, %v: %v", filename, err, err))
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
		s3.logger.Error(fmt.Sprintf("Unable to save %q to %q, %v: %v", filename, s3.opts.Bucket, err, err))
		return err
	}

	s3.logger.Info(fmt.Sprintf("Successfully saved %q to %q\n", filename, s3.opts.Bucket))
	return nil
}
