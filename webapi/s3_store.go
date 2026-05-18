package main

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3StoreConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	Region    string
}

type S3Store struct {
	client *minio.Client
	bucket string
}

func NewS3Store(ctx context.Context, cfg S3StoreConfig) (*S3Store, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("empty s3 endpoint")
	}
	if cfg.AccessKey == "" {
		return nil, fmt.Errorf("empty s3 access key")
	}
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("empty s3 secret key")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("empty s3 bucket")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	endpoint, useSSL, err := normalizeS3Endpoint(cfg.Endpoint, cfg.UseSSL)
	if err != nil {
		return nil, err
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: useSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, err
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("s3 bucket %q does not exist", cfg.Bucket)
	}

	return &S3Store{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

func (s *S3Store) OpenObject(ctx context.Context, key string) (*minio.Object, minio.ObjectInfo, error) {
	objectKey, err := normalizeS3ObjectKey(key)
	if err != nil {
		return nil, minio.ObjectInfo{}, err
	}

	info, err := s.client.StatObject(ctx, s.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, minio.ObjectInfo{}, err
	}

	object, err := s.client.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, minio.ObjectInfo{}, err
	}

	return object, info, nil
}

func normalizeS3Endpoint(endpoint string, useSSL bool) (string, bool, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", false, fmt.Errorf("empty s3 endpoint")
	}

	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		u, err := url.Parse(endpoint)
		if err != nil {
			return "", false, err
		}
		if u.Host == "" {
			return "", false, fmt.Errorf("invalid s3 endpoint %q", endpoint)
		}
		if u.Path != "" && u.Path != "/" {
			return "", false, fmt.Errorf("s3 endpoint must not contain path: %q", endpoint)
		}

		switch u.Scheme {
		case "http":
			return u.Host, false, nil
		case "https":
			return u.Host, true, nil
		default:
			return "", false, fmt.Errorf("unsupported s3 endpoint scheme %q", u.Scheme)
		}
	}

	return endpoint, useSSL, nil
}

func normalizeS3ObjectKey(key string) (string, error) {
	key = strings.ReplaceAll(key, "\\", "/")
	key = strings.TrimPrefix(key, "/")
	key = path.Clean(key)

	if key == "." || key == "" {
		return "", fmt.Errorf("empty s3 object key")
	}
	if key == ".." || strings.HasPrefix(key, "../") {
		return "", fmt.Errorf("unsafe s3 object key %q", key)
	}

	return key, nil
}
