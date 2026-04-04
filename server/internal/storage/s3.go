package storage

import (
	"bytes"
	"context"
	"fmt"

	"github.com/klauspost/compress/zstd"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Store implements ObjectStore using an S3-compatible API.
type S3Store struct {
	client *minio.Client
	bucket string
}

// NewS3Store creates a new S3 store using the MinIO client.
func NewS3Store(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*S3Store, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("s3: create client: %w", err)
	}

	return &S3Store{
		client: client,
		bucket: bucket,
	}, nil
}

// Upload compresses data with zstd and uploads it to S3.
func (s *S3Store) Upload(ctx context.Context, key string, data []byte) error {
	compressed, err := compressZstd(data)
	if err != nil {
		return fmt.Errorf("s3: compress: %w", err)
	}

	reader := bytes.NewReader(compressed)
	_, err = s.client.PutObject(ctx, s.bucket, key, reader, int64(len(compressed)), minio.PutObjectOptions{
		ContentType: "application/zstd",
	})
	if err != nil {
		return fmt.Errorf("s3: upload %s: %w", key, err)
	}
	return nil
}

// EnsureBucket creates the bucket if it does not exist.
func (s *S3Store) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("s3: check bucket: %w", err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("s3: create bucket: %w", err)
		}
	}
	return nil
}

func compressZstd(data []byte) ([]byte, error) {
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, err
	}
	return encoder.EncodeAll(data, nil), nil
}
