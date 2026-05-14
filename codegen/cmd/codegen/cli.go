package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gulyaev-iv/go-cloud-des/codegen/builder"
	"github.com/gulyaev-iv/go-cloud-des/codegen/generator"
)

func runGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)

	inPath := fs.String("in", "", "input DSL file")
	outPath := fs.String("out", "-", "output Go source file, or - for stdout")
	printHash := fs.Bool("print-hash", false, "print model hash to stderr")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *inPath == "" {
		return fmt.Errorf("missing required -in")
	}

	result, err := generateFromFile(*inPath)
	if err != nil {
		return err
	}

	if *printHash {
		fmt.Fprintln(os.Stderr, result.Hash)
	}

	if *outPath == "-" {
		_, err = os.Stdout.Write(result.Source)
		return err
	}

	if err := ensureParentDir(*outPath); err != nil {
		return err
	}

	return os.WriteFile(*outPath, result.Source, 0o644)
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)

	inPath := fs.String("in", "", "input DSL file")
	outPath := fs.String("out", "", "output model binary")
	goos := fs.String("goos", runtime.GOOS, "target GOOS")
	goarch := fs.String("goarch", runtime.GOARCH, "target GOARCH")
	moduleDir := fs.String("module-dir", ".", "local path to codegen module")
	workDirRoot := fs.String("work-dir", "", "temporary build work directory root")
	buildTimeout := fs.Duration("build-timeout", 120*time.Second, "go build timeout")
	keepWorkDir := fs.Bool("keep-workdir", false, "do not delete temporary build directory")
	printHash := fs.Bool("print-hash", false, "print model hash to stderr")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *inPath == "" {
		return fmt.Errorf("missing required -in")
	}
	if *outPath == "" {
		return fmt.Errorf("missing required -out")
	}
	if *goos == "" {
		return fmt.Errorf("empty -goos")
	}
	if *goarch == "" {
		return fmt.Errorf("empty -goarch")
	}

	result, err := generateFromFile(*inPath)
	if err != nil {
		return err
	}

	if *printHash {
		fmt.Fprintln(os.Stderr, result.Hash)
	}

	buildResult, err := builder.Build(context.Background(), builder.Request{
		Source:      result.Source,
		OutputPath:  *outPath,
		GOOS:        *goos,
		GOARCH:      *goarch,
		ModuleDir:   *moduleDir,
		WorkDirRoot: *workDirRoot,
		Timeout:     *buildTimeout,
		KeepWorkDir: *keepWorkDir,
	})
	if err != nil {
		if buildResult != nil && len(buildResult.BuildLog) > 0 {
			fmt.Fprintln(os.Stderr, string(buildResult.BuildLog))
		}
		return err
	}

	if *keepWorkDir {
		fmt.Fprintln(os.Stderr, "workdir:", buildResult.WorkDir)
	}

	return nil
}

func generateFromFile(path string) (*generator.Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return generator.GenerateAutoHash(data)
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}

	return os.MkdirAll(dir, 0o755)
}
