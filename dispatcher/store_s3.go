package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3StoreConfig struct {
	Endpoint     string
	AccessKey    string
	SecretKey    string
	Bucket       string
	UseSSL       bool
	Region       string
	Prefix       string
	CreateBucket bool
}

type S3Store struct {
	Client *minio.Client
	Bucket string
	Prefix string
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

	prefix, err := normalizeS3Prefix(cfg.Prefix)
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
		if !cfg.CreateBucket {
			return nil, fmt.Errorf("s3 bucket %q does not exist", cfg.Bucket)
		}

		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{
			Region: cfg.Region,
		}); err != nil {
			return nil, err
		}
	}

	return &S3Store{
		Client: client,
		Bucket: cfg.Bucket,
		Prefix: prefix,
	}, nil
}

func (s *S3Store) GetFile(ctx context.Context, key string, dstPath string, expectedSHA256 string) (*ArtifactRef, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	objectKey, err := normalizeS3ObjectKey(key)
	if err != nil {
		return nil, err
	}

	info, err := s.Client.StatObject(ctx, s.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}

	object, err := s.Client.GetObject(ctx, s.Bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return nil, err
	}

	tmpPath := dstPath + ".tmp"
	dst, err := os.Create(tmpPath)
	if err != nil {
		return nil, err
	}

	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(dst, hash), object)
	closeErr := dst.Close()

	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return nil, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return nil, closeErr
	}

	sum := hex.EncodeToString(hash.Sum(nil))
	if expectedSHA256 != "" && !strings.EqualFold(sum, expectedSHA256) {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("sha256 mismatch for %s: expected %s, got %s", objectKey, expectedSHA256, sum)
	}

	if err := os.Rename(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	contentType := info.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return &ArtifactRef{
		Key:         objectKey,
		SHA256:      sum,
		Size:        size,
		ContentType: contentType,
	}, nil
}

func (s *S3Store) PutFile(ctx context.Context, key string, srcPath string, contentType string) (*ArtifactRef, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	objectKey, err := s.objectKeyForWrite(key)
	if err != nil {
		return nil, err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	stat, err := src.Stat()
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("source path is directory: %s", srcPath)
	}

	hash := sha256.New()
	size, err := io.Copy(hash, src)
	if err != nil {
		return nil, err
	}
	if size != stat.Size() {
		return nil, fmt.Errorf("source file size changed while hashing: expected %d bytes, read %d bytes", stat.Size(), size)
	}

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = s.Client.PutObject(
		ctx,
		s.Bucket,
		objectKey,
		src,
		stat.Size(),
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	if err != nil {
		return nil, err
	}

	return &ArtifactRef{
		Key:         objectKey,
		SHA256:      hex.EncodeToString(hash.Sum(nil)),
		Size:        stat.Size(),
		ContentType: contentType,
	}, nil
}

func (s *S3Store) objectKeyForWrite(key string) (string, error) {
	cleanKey, err := normalizeS3ObjectKey(key)
	if err != nil {
		return "", err
	}

	if s.Prefix == "" {
		return cleanKey, nil
	}

	return s.Prefix + "/" + cleanKey, nil
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

func normalizeS3Prefix(prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", nil
	}

	return normalizeS3ObjectKey(prefix)
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
