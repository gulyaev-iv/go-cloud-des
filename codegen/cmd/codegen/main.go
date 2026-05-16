package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	var err error

	switch os.Args[1] {
	case "generate":
		err = runGenerate(os.Args[2:])

	case "build":
		err = runBuild(os.Args[2:])

	case "serve":
		err = runServe(os.Args[2:])

	default:
		printUsage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  codegen generate -in model.dsl -out generated/main.go")
	fmt.Fprintln(os.Stderr, "  codegen generate -in model.dsl -out -")
	fmt.Fprintln(os.Stderr, "  codegen build -in model.dsl -out ./bin/model")
	fmt.Fprintln(os.Stderr, "  codegen build -in model.dsl -out ./bin/model-linux-amd64 -goos linux -goarch amd64")
	fmt.Fprintln(os.Stderr, "  codegen serve -workers 4 -nats-url nats://localhost:4222 -artifact-store fs -artifact-dir ./artifacts")
	fmt.Fprintln(os.Stderr, "  codegen serve -workers 4 -nats-url nats://localhost:4222 -artifact-store s3 -s3-endpoint localhost:9000 -s3-bucket cloud-des-artifacts")
	fmt.Fprintln(os.Stderr, "  codegen serve -workers 4 -nats-url nats://localhost:4222 -artifact-store s3 -s3-endpoint localhost:9000 -s3-bucket cloud-des-artifacts -metrics-subject codegen.metrics.task")
}
