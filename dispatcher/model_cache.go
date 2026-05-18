package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type CachedModel struct {
	ModelHash    string
	BinaryKey    string
	BinarySHA256 string
	LocalPath    string
	DownloadedAt time.Time
	LastUsedAt   time.Time
}

type ModelCache struct {
	workDir string
	store   *S3Store

	mu     sync.Mutex
	models map[string]CachedModel
}

func NewModelCache(workDir string, store *S3Store) *ModelCache {
	return &ModelCache{
		workDir: workDir,
		store:   store,
		models:  make(map[string]CachedModel),
	}
}

func (c *ModelCache) GetOrDownload(ctx context.Context, modelHash string, binaryKey string, binarySHA256 string) (CachedModel, error) {
	if modelHash == "" {
		return CachedModel{}, fmt.Errorf("empty model hash")
	}
	if binaryKey == "" {
		return CachedModel{}, fmt.Errorf("empty binary key")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	expectedSHA := strings.ToLower(strings.TrimSpace(binarySHA256))

	if cached, ok := c.models[modelHash]; ok {
		if cached.LocalPath != "" && fileExists(cached.LocalPath) {
			if expectedSHA != "" && strings.EqualFold(cached.BinarySHA256, expectedSHA) {
				cached.LastUsedAt = time.Now()
				c.models[modelHash] = cached
				log.Printf(
					"model binary selected: model_hash=%s source=CACHED key=%s sha256=%s",
					modelHash,
					cached.BinaryKey,
					cached.BinarySHA256,
				)
				return cached, nil
			}

			if expectedSHA == "" && cached.BinaryKey == binaryKey {
				cached.LastUsedAt = time.Now()
				c.models[modelHash] = cached
				log.Printf(
					"model binary selected: model_hash=%s source=CACHED key=%s sha256=%s",
					modelHash,
					cached.BinaryKey,
					cached.BinarySHA256,
				)
				return cached, nil
			}
		}
	}

	modelDir := filepath.Join(c.workDir, "models", safePathName(modelHash))
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		return CachedModel{}, err
	}

	localPath := filepath.Join(modelDir, modelBinaryFileName(runtime.GOOS))

	ref, err := c.store.GetFile(ctx, binaryKey, localPath, expectedSHA)
	if err != nil {
		return CachedModel{}, err
	}

	now := time.Now()
	cached := CachedModel{
		ModelHash:    modelHash,
		BinaryKey:    binaryKey,
		BinarySHA256: ref.SHA256,
		LocalPath:    localPath,
		DownloadedAt: now,
		LastUsedAt:   now,
	}

	c.models[modelHash] = cached

	if err := os.Chmod(localPath, 0o755); err != nil {
		return CachedModel{}, err
	}

	log.Printf(
		"model binary selected: model_hash=%s source=S3 key=%s sha256=%s size=%d",
		modelHash,
		binaryKey,
		ref.SHA256,
		ref.Size,
	)
	return cached, nil
}

func (c *ModelCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.models)
}

func fileExists(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !stat.IsDir()
}

func (c *ModelCache) ClearWorkDir() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.RemoveAll(c.workDir); err != nil {
		return err
	}

	c.models = make(map[string]CachedModel)

	log.Printf("dispatcher work dir cleared: path=%s", c.workDir)

	return nil
}
