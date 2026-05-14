package builder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const CodegenModulePath = "github.com/gulyaev-iv/go-cloud-des/codegen"

type Request struct {
	Source      []byte
	OutputPath  string
	GOOS        string
	GOARCH      string
	ModuleDir   string
	WorkDirRoot string
	Timeout     time.Duration
	KeepWorkDir bool
}

type Result struct {
	WorkDir    string
	GoModPath  string
	SourcePath string
	BinaryPath string
	BuildLog   []byte
}

func Build(ctx context.Context, req Request) (*Result, error) {
	if len(req.Source) == 0 {
		return nil, fmt.Errorf("empty source")
	}

	if req.OutputPath == "" {
		return nil, fmt.Errorf("empty output path")
	}

	if req.GOOS == "" {
		req.GOOS = runtime.GOOS
	}

	if req.GOARCH == "" {
		req.GOARCH = runtime.GOARCH
	}

	if req.ModuleDir == "" {
		req.ModuleDir = "."
	}

	absModuleDir, err := filepath.Abs(req.ModuleDir)
	if err != nil {
		return nil, err
	}

	if err := ValidateCodegenModuleDir(absModuleDir); err != nil {
		return nil, err
	}

	workDir, err := makeWorkDir(req.WorkDirRoot)
	if err != nil {
		return nil, err
	}

	if !req.KeepWorkDir {
		defer os.RemoveAll(workDir)
	}

	result := &Result{
		WorkDir:    workDir,
		GoModPath:  filepath.Join(workDir, "go.mod"),
		SourcePath: filepath.Join(workDir, "main.go"),
		BinaryPath: req.OutputPath,
	}

	if err := writeBuildWorkDir(result, absModuleDir, req.Source); err != nil {
		return result, err
	}

	if err := ensureParentDir(req.OutputPath); err != nil {
		return result, err
	}

	absOutputPath, err := filepath.Abs(req.OutputPath)
	if err != nil {
		return result, err
	}

	result.BinaryPath = absOutputPath

	buildLog, err := runGoBuild(ctx, workDir, absOutputPath, req.GOOS, req.GOARCH, req.Timeout)
	result.BuildLog = buildLog
	if err != nil {
		return result, err
	}

	return result, nil
}

func makeWorkDir(root string) (string, error) {
	if root == "" {
		return os.MkdirTemp("", "cloud-des-codegen-build-*")
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}

	return os.MkdirTemp(root, "build-*")
}

func writeBuildWorkDir(result *Result, moduleDir string, source []byte) error {
	goMod := fmt.Sprintf(`module cloud-des-generated-model

go 1.26.2

require %s v0.0.0

replace %s => %s
`, CodegenModulePath, CodegenModulePath, filepath.ToSlash(moduleDir))

	if err := os.WriteFile(result.GoModPath, []byte(goMod), 0o644); err != nil {
		return err
	}

	return os.WriteFile(result.SourcePath, source, 0o644)
}

func runGoBuild(ctx context.Context, workDir string, outputPath string, goos string, goarch string, timeout time.Duration) ([]byte, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "go", "build", "-o", outputPath, ".")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"GOOS="+goos,
		"GOARCH="+goarch,
		"CGO_ENABLED=0",
	)

	var log bytes.Buffer
	cmd.Stdout = &log
	cmd.Stderr = &log

	err := cmd.Run()
	buildLog := log.Bytes()

	if ctx.Err() == context.DeadlineExceeded {
		return buildLog, fmt.Errorf("go build timeout")
	}

	if err != nil {
		msg := strings.TrimSpace(string(buildLog))
		if msg == "" {
			return buildLog, err
		}
		return buildLog, fmt.Errorf("%w: %s", err, msg)
	}

	return buildLog, nil
}

func ValidateCodegenModuleDir(moduleDir string) error {
	data, err := os.ReadFile(filepath.Join(moduleDir, "go.mod"))
	if err != nil {
		return fmt.Errorf("invalid module dir: %w", err)
	}

	expected := "module " + CodegenModulePath
	if !strings.Contains(string(data), expected) {
		return fmt.Errorf("invalid module dir %q: go.mod must contain %q", moduleDir, expected)
	}

	return nil
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}

	return os.MkdirAll(dir, 0o755)
}
