package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Store interface {
	PutBytes(ctx context.Context, key string, data []byte, contentType string) (*ArtifactRef, error)
	PutFile(ctx context.Context, key string, path string, contentType string) (*ArtifactRef, error)
}

type FSStore struct {
	RootDir string
	URIBase string
}

func NewFSStore(rootDir string) (*FSStore, error) {
	if rootDir == "" {
		return nil, fmt.Errorf("empty artifact dir")
	}

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	return &FSStore{
		RootDir: absRoot,
		URIBase: fileURI(absRoot),
	}, nil
}

func (s *FSStore) PutBytes(ctx context.Context, key string, data []byte, contentType string) (*ArtifactRef, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	path, err := s.objectPath(key)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}

	sum := sha256.Sum256(data)

	return &ArtifactRef{
		URI:         s.objectURI(key),
		Key:         normalizeObjectKey(key),
		SHA256:      hex.EncodeToString(sum[:]),
		Size:        int64(len(data)),
		ContentType: contentType,
	}, nil
}

func (s *FSStore) PutFile(ctx context.Context, key string, srcPath string, contentType string) (*ArtifactRef, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	dstPath, err := s.objectPath(key)
	if err != nil {
		return nil, err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return nil, err
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	hash := sha256.New()
	w := io.MultiWriter(dst, hash)

	size, err := io.Copy(w, src)
	if err != nil {
		return nil, err
	}

	return &ArtifactRef{
		URI:         s.objectURI(key),
		Key:         normalizeObjectKey(key),
		SHA256:      hex.EncodeToString(hash.Sum(nil)),
		Size:        size,
		ContentType: contentType,
	}, nil
}

func (s *FSStore) objectPath(key string) (string, error) {
	cleanKey := normalizeObjectKey(key)
	if cleanKey == "" {
		return "", fmt.Errorf("empty artifact key")
	}

	if strings.HasPrefix(cleanKey, "../") || cleanKey == ".." || filepath.IsAbs(cleanKey) {
		return "", fmt.Errorf("unsafe artifact key %q", key)
	}

	return filepath.Join(s.RootDir, filepath.FromSlash(cleanKey)), nil
}

func (s *FSStore) objectURI(key string) string {
	cleanKey := normalizeObjectKey(key)
	return strings.TrimRight(s.URIBase, "/") + "/" + cleanKey
}

func normalizeObjectKey(key string) string {
	key = filepath.ToSlash(key)
	key = strings.TrimPrefix(key, "/")
	return filepath.Clean(key)
}

func fileURI(path string) string {
	path = filepath.ToSlash(path)

	u := url.URL{
		Scheme: "file",
		Path:   path,
	}

	return u.String()
}
