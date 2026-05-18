package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Reporter struct {
	journalDir    string
	client        *GRPCControlPlaneClient
	retryInterval time.Duration
	sendTimeout   time.Duration

	signal chan struct{}
	wg     sync.WaitGroup
}

func NewReporter(journalDir string, client *GRPCControlPlaneClient, retryInterval time.Duration, sendTimeout time.Duration) (*Reporter, error) {
	if journalDir == "" {
		return nil, fmt.Errorf("empty journal dir")
	}
	if client == nil {
		return nil, fmt.Errorf("nil control plane client")
	}
	if retryInterval <= 0 {
		retryInterval = 10 * time.Second
	}
	if sendTimeout <= 0 {
		sendTimeout = 5 * time.Second
	}

	if err := os.MkdirAll(journalDir, 0o755); err != nil {
		return nil, fmt.Errorf("create journal dir: %w", err)
	}

	return &Reporter{
		journalDir:    journalDir,
		client:        client,
		retryInterval: retryInterval,
		sendTimeout:   sendTimeout,
		signal:        make(chan struct{}, 1),
	}, nil
}

func (r *Reporter) Start(ctx context.Context) {
	r.wg.Add(1)
	go r.loop(ctx)
}

func (r *Reporter) Stop() {
	r.wg.Wait()
}

func (r *Reporter) Submit(result ExperimentResult) {
	if err := r.persist(result); err != nil {
		log.Printf(
			"reporter persist failed: model_hash=%s experiment_id=%s error=%v",
			result.ModelHash,
			result.ExperimentID,
			err,
		)
		return
	}

	select {
	case r.signal <- struct{}{}:
	default:
	}
}

func (r *Reporter) persist(result ExperimentResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	name := reportFileName(result.ModelHash, result.ExperimentID)
	tmp := filepath.Join(r.journalDir, name+".tmp")
	final := filepath.Join(r.journalDir, name)

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, final)
}

func (r *Reporter) loop(ctx context.Context) {
	defer r.wg.Done()

	r.flush(ctx)

	ticker := time.NewTicker(r.retryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.flush(ctx)
		case <-r.signal:
			r.flush(ctx)
		}
	}
}

func (r *Reporter) flush(ctx context.Context) {
	entries, err := os.ReadDir(r.journalDir)
	if err != nil {
		log.Printf("reporter scan journal failed: dir=%s error=%v", r.journalDir, err)
		return
	}

	for _, entry := range entries {
		if ctx.Err() != nil {
			return
		}
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		path := filepath.Join(r.journalDir, name)
		if err := r.flushOne(ctx, path); err != nil {
			log.Printf("reporter send failed (will retry): file=%s error=%v", name, err)
		}
	}
}

func (r *Reporter) flushOne(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var result ExperimentResult
	if err := json.Unmarshal(data, &result); err != nil {
		log.Printf("reporter cannot parse journal file, deleting: file=%s error=%v", path, err)
		if rmErr := os.Remove(path); rmErr != nil {
			log.Printf("reporter remove corrupt file failed: file=%s error=%v", path, rmErr)
		}
		return nil
	}

	sendCtx, cancel := context.WithTimeout(ctx, r.sendTimeout)
	defer cancel()

	if err := r.client.ReportExperimentResult(sendCtx, result); err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		log.Printf("reporter remove journal file failed: file=%s error=%v", path, err)
	}

	log.Printf(
		"reporter delivered experiment result: model_hash=%s experiment_id=%s status=%s",
		result.ModelHash,
		result.ExperimentID,
		result.Status,
	)

	return nil
}

func reportFileName(modelHash string, experimentID string) string {
	replacer := strings.NewReplacer("/", "_", ":", "_", "\\", "_", "..", "_")
	return replacer.Replace(modelHash+"__"+experimentID) + ".json"
}
