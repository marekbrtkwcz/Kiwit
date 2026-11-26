package storage

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config holds settings for S3-compatible object storage (works with MinIO).
type S3Config struct {
	Endpoint     string // e.g. http://localhost:9000 (empty = AWS default)
	Region       string // e.g. us-east-1
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool // required for MinIO
}

// S3Storage stores files in an S3-compatible bucket.
type S3Storage struct {
	client *s3.Client
	bucket string
}

func NewS3(cfg S3Config) *S3Storage {
	creds := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")
	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: creds,
	}
	opts := func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	}
	return &S3Storage{
		client: s3.NewFromConfig(awsCfg, opts),
		bucket: cfg.Bucket,
	}
}

func (s *S3Storage) Put(ctx context.Context, key string, r io.Reader, size int64) (string, error) {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          r,
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return "", err
	}
	return key, nil
}

func (s *S3Storage) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (s *S3Storage) Delete(ctx context.Context, path string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	return err
}
