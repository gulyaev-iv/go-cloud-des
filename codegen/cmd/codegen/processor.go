package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gulyaev-iv/go-cloud-des/codegen/builder"
	"github.com/gulyaev-iv/go-cloud-des/codegen/generator"
)

type ProcessConfig struct {
	ModuleDir     string
	WorkDirRoot   string
	DefaultGOOS   string
	DefaultGOARCH string
	BuildTimeout  time.Duration
	KeepWorkDir   bool
}

func processTask(ctx context.Context, cfg ProcessConfig, store Store, task Task) ProcessOutput {
	totalStart := time.Now()

	result := TaskResult{
		RequestID: task.RequestID,
		ModelHash: task.ModelHash,
		Mode:      task.Mode,
		GOOS:      task.GOOS,
		GOARCH:    task.GOARCH,
	}

	metrics := TaskMetrics{
		RequestID: task.RequestID,
		ModelHash: task.ModelHash,
		Mode:      task.Mode,
		GOOS:      task.GOOS,
		GOARCH:    task.GOARCH,
	}

	finish := func(result TaskResult) ProcessOutput {
		completeTaskMetrics(&metrics, totalStart, result)
		return ProcessOutput{
			Result:  result,
			Metrics: metrics,
		}
	}

	fail := func(stage string, code string, err error) ProcessOutput {
		return finish(failResult(result, stage, code, err))
	}

	if err := normalizeTask(&task, &result, cfg); err != nil {
		return fail(stageValidateRequest, errorBadRequest, err)
	}

	generateStart := time.Now()
	genResult, err := generator.GenerateWithHash(task.ModelHash, []byte(task.DSL))
	metrics.GenerateMS = durationMS(time.Since(generateStart))
	if err != nil {
		return fail(stageGenerate, errorGenerate, err)
	}

	result.ModelHash = genResult.Hash

	storeSource := boolOrDefault(task.StoreSource, true)
	if storeSource {
		storeSourceStart := time.Now()
		ref, err := store.PutBytes(
			ctx,
			sourceObjectKey(task.ModelHash),
			genResult.Source,
			"text/x-go; charset=utf-8",
		)
		metrics.StoreSourceMS = durationMS(time.Since(storeSourceStart))
		if err != nil {
			return fail(stageStoreSource, errorStoreSource, err)
		}

		result.Source = ref
	}

	if task.Mode == taskModeGenerate {
		result.OK = true
		return finish(result)
	}

	storeBinary := boolOrDefault(task.StoreBinary, true)

	outputDir, err := os.MkdirTemp(cfg.WorkDirRoot, "cloud-des-codegen-bin-*")
	if err != nil {
		return fail(stageBuild, errorBuild, err)
	}
	defer os.RemoveAll(outputDir)

	outputPath := filepath.Join(outputDir, binaryFileName(task.GOOS))

	timeout := cfg.BuildTimeout
	if task.BuildTimeoutSec > 0 {
		timeout = time.Duration(task.BuildTimeoutSec) * time.Second
	}

	buildStart := time.Now()
	buildResult, err := builder.Build(ctx, builder.Request{
		Source:      genResult.Source,
		OutputPath:  outputPath,
		GOOS:        task.GOOS,
		GOARCH:      task.GOARCH,
		ModuleDir:   cfg.ModuleDir,
		WorkDirRoot: cfg.WorkDirRoot,
		Timeout:     timeout,
		KeepWorkDir: cfg.KeepWorkDir,
	})
	metrics.BuildMS = durationMS(time.Since(buildStart))

	if buildResult != nil && len(buildResult.BuildLog) > 0 {
		storeBuildLogStart := time.Now()
		ref, storeErr := store.PutBytes(
			ctx,
			buildLogObjectKey(task.ModelHash, task.GOOS, task.GOARCH),
			buildResult.BuildLog,
			"text/plain; charset=utf-8",
		)
		metrics.StoreBuildLogMS = durationMS(time.Since(storeBuildLogStart))

		if storeErr != nil && err == nil {
			return fail(stageStoreBuildLog, errorStoreBuildLog, storeErr)
		}

		if storeErr == nil {
			result.BuildLog = ref
		}
	}

	if err != nil {
		return fail(stageBuild, errorBuild, err)
	}

	if storeBinary {
		storeBinaryStart := time.Now()
		ref, err := store.PutFile(
			ctx,
			binaryObjectKey(task.ModelHash, task.GOOS, task.GOARCH),
			buildResult.BinaryPath,
			"application/octet-stream",
		)
		metrics.StoreBinaryMS = durationMS(time.Since(storeBinaryStart))
		if err != nil {
			return fail(stageStoreBinary, errorStoreBinary, err)
		}

		result.Binary = ref
	}

	result.OK = true
	return finish(result)
}

func normalizeTask(task *Task, result *TaskResult, cfg ProcessConfig) error {
	if task.RequestID == "" {
		return fmt.Errorf("empty request_id")
	}
	if task.ModelHash == "" {
		return fmt.Errorf("empty model_hash")
	}
	if task.DSL == "" {
		return fmt.Errorf("empty dsl")
	}

	if task.Mode == "" {
		task.Mode = taskModeBuild
	}
	if task.Mode != taskModeGenerate && task.Mode != taskModeBuild {
		return fmt.Errorf("unknown mode %q", task.Mode)
	}

	if task.Mode == taskModeBuild {
		if task.GOOS == "" {
			task.GOOS = cfg.DefaultGOOS
		}
		if task.GOOS == "" {
			task.GOOS = runtime.GOOS
		}

		if task.GOARCH == "" {
			task.GOARCH = cfg.DefaultGOARCH
		}
		if task.GOARCH == "" {
			task.GOARCH = runtime.GOARCH
		}
	}

	result.Mode = task.Mode
	result.GOOS = task.GOOS
	result.GOARCH = task.GOARCH

	return nil
}

func failResult(result TaskResult, stage string, code string, err error) TaskResult {
	result.OK = false
	result.Stage = stage
	result.ErrorCode = code
	if err != nil {
		result.Message = err.Error()
	}
	return result
}

func boolOrDefault(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

func sourceObjectKey(hash string) string {
	return fmt.Sprintf("models/%s/source/main.go", hash)
}

func binaryObjectKey(hash string, goos string, goarch string) string {
	return fmt.Sprintf("models/%s/bin/%s/%s/%s", hash, goos, goarch, binaryFileName(goos))
}

func buildLogObjectKey(hash string, goos string, goarch string) string {
	return fmt.Sprintf("models/%s/logs/%s/%s/build.log", hash, goos, goarch)
}

func binaryFileName(goos string) string {
	if goos == "windows" {
		return "model.exe"
	}

	return "model"
}
