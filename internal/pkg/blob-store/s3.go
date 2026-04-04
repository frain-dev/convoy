package blobstore

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	log "github.com/frain-dev/convoy/pkg/logger"
)

// S3Client implements BlobStore for AWS S3 (and S3-compatible) backends.
type S3Client struct {
	session *session.Session
	opts    BlobStoreOptions
	logger  log.Logger
}

// NewS3Client creates a new S3-backed BlobStore.
func NewS3Client(opts BlobStoreOptions, logger log.Logger) (BlobStore, error) {
	sess, err := session.NewSession(&aws.Config{
		S3ForcePathStyle: aws.Bool(true),
		Region:           aws.String(opts.Region),
		Endpoint:         aws.String(opts.Endpoint),
		Credentials:      credentials.NewStaticCredentials(opts.AccessKey, opts.SecretKey, opts.SessionToken),
	})
	if err != nil {
		return nil, err
	}

	return &S3Client{
		session: sess,
		opts:    opts,
		logger:  logger,
	}, nil
}

// Upload streams data directly to S3 via multipart upload.
func (s3c *S3Client) Upload(ctx context.Context, key string, r io.Reader) error {
	name := key
	if s3c.opts.Prefix != "" {
		name = s3c.opts.Prefix + "/" + key
	}

	uploader := s3manager.NewUploader(s3c.session)
	_, err := uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(s3c.opts.Bucket),
		Key:    aws.String(name),
		Body:   r,
	})
	if err != nil {
		s3c.logger.Error(fmt.Sprintf("failed to upload %q to %q: %v", name, s3c.opts.Bucket, err))
		return err
	}

	s3c.logger.Info(fmt.Sprintf("uploaded %q to %q", name, s3c.opts.Bucket))
	return nil
}
