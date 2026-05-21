package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type ModelRunConfig struct {
	ExperimentID string
	ModelHash    string
	BinaryPath   string

	SetVars    []SetVar
	StopRule   *StopRule
	Metrics    []string
	MetricStep float64
	Trace      *CSVTraceWriter

	MemoryLimitBytes uint64

	CgroupEnabled bool
	CgroupRoot    string
	CgroupParent  string
}

type ModelRunResult struct {
	Status       string
	FinishReason string
	ExitCode     int
	ErrorMessage string
	FinalMetrics map[string]string
	StatCount    uint64

	StartedAtUnixMS  int64
	FinishedAtUnixMS int64
	DurationMS       int64

	MemoryLimitBytes uint64
	PeakMemoryBytes  uint64

	StderrTail string
}

func RunModelProcess(ctx context.Context, cfg ModelRunConfig) ModelRunResult {
	result := ModelRunResult{
		Status:           statusFailed,
		ExitCode:         -1,
		FinalMetrics:     map[string]string{},
		MemoryLimitBytes: cfg.MemoryLimitBytes,
	}

	cmd := exec.CommandContext(ctx, cfg.BinaryPath)

	var cg *ExperimentCgroup

	if cfg.CgroupEnabled && cfg.MemoryLimitBytes > 0 {
		var err error

		cg, err = NewExperimentCgroup(ExperimentCgroupConfig{
			Root:             cfg.CgroupRoot,
			Parent:           cfg.CgroupParent,
			ModelHash:        cfg.ModelHash,
			ExperimentID:     cfg.ExperimentID,
			MemoryLimitBytes: cfg.MemoryLimitBytes,
		})
		if err != nil {
			result.ErrorMessage = "create cgroup: " + err.Error()
			return result
		}

		defer func() {
			_ = cg.Close()
		}()
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		result.ErrorMessage = "stdin pipe: " + err.Error()
		return result
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result.ErrorMessage = "stdout pipe: " + err.Error()
		return result
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.ErrorMessage = "stderr pipe: " + err.Error()
		return result
	}

	if err := cmd.Start(); err != nil {
		result.ErrorMessage = "start process: " + err.Error()
		return result
	}

	if cg != nil {
		if err := cg.AddProcess(cmd.Process.Pid); err != nil {
			result.ErrorMessage = "add process to cgroup: " + err.Error()

			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}

			startedAt := time.Now()
			result.StartedAtUnixMS = startedAt.UnixMilli()

			stderrCh := captureLimited(stderr, 64*1024)
			waitProcess(cmd, &result)
			finishRunResult(&result, startedAt, stderrCh, cg)

			return result
		}
	}

	startedAt := time.Now()
	result.StartedAtUnixMS = startedAt.UnixMilli()

	stderrCh := captureLimited(stderr, 64*1024)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	fail := func(message string) {
		result.Status = statusFailed
		result.ErrorMessage = message
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}

	helloLine, ok := scanNonEmptyLine(scanner)
	if !ok {
		if err := scanner.Err(); err != nil {
			fail("read HELLO: " + err.Error())
		} else {
			fail("process closed stdout before HELLO")
		}
		waitProcess(cmd, &result)
		finishRunResult(&result, startedAt, stderrCh, cg)
		return result
	}

	helloHash, err := parseHello(helloLine)
	if err != nil {
		fail(err.Error())
		waitProcess(cmd, &result)
		finishRunResult(&result, startedAt, stderrCh, cg)
		return result
	}
	if helloHash != cfg.ModelHash {
		fail(fmt.Sprintf("model hash mismatch: expected %s, got %s", cfg.ModelHash, helloHash))
		waitProcess(cmd, &result)
		finishRunResult(&result, startedAt, stderrCh, cg)
		return result
	}

	writer := bufio.NewWriter(stdin)
	if err := writeModelConfig(writer, cfg); err != nil {
		fail("write model config: " + err.Error())
		_ = stdin.Close()
		waitProcess(cmd, &result)
		finishRunResult(&result, startedAt, stderrCh, cg)
		return result
	}
	if err := writer.Flush(); err != nil {
		fail("flush model config: " + err.Error())
		_ = stdin.Close()
		waitProcess(cmd, &result)
		finishRunResult(&result, startedAt, stderrCh, cg)
		return result
	}
	_ = stdin.Close()

	if !waitReady(scanner, fail) {
		waitProcess(cmd, &result)
		finishRunResult(&result, startedAt, stderrCh, cg)
		return result
	}

	receivedEnd := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "STAT "):
			stat, err := parseStatLine(line)
			if err != nil {
				fail("parse STAT: " + err.Error())
				waitProcess(cmd, &result)
				finishRunResult(&result, startedAt, stderrCh, cg)
				return result
			}

			result.FinalMetrics = stat
			result.StatCount++

			if cfg.Trace != nil {
				if err := cfg.Trace.WriteStat(stat); err != nil {
					fail("write metrics csv: " + err.Error())
					waitProcess(cmd, &result)
					finishRunResult(&result, startedAt, stderrCh, cg)
					return result
				}
			}

		case strings.HasPrefix(line, "END"):
			result.FinishReason = strings.TrimSpace(strings.TrimPrefix(line, "END"))
			receivedEnd = true

		case strings.HasPrefix(line, "ERR "):
			fail("model returned " + line)
			waitProcess(cmd, &result)
			finishRunResult(&result, startedAt, stderrCh, cg)
			return result

		default:
			fail("unexpected model line: " + line)
			waitProcess(cmd, &result)
			finishRunResult(&result, startedAt, stderrCh, cg)
			return result
		}

		if receivedEnd {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		fail("read model stdout: " + err.Error())
		waitProcess(cmd, &result)
		finishRunResult(&result, startedAt, stderrCh, cg)
		return result
	}

	waitErr := waitProcess(cmd, &result)

	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			result.Status = statusCanceled
			result.FinishReason = "user_canceled"
			result.ErrorMessage = ""
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Status = statusFailed
			result.FinishReason = "timeout"
			result.ErrorMessage = ctx.Err().Error()
		} else {
			result.Status = statusFailed
			result.ErrorMessage = ctx.Err().Error()
		}

		finishRunResult(&result, startedAt, stderrCh, cg)
		return result
	}

	if waitErr != nil && result.ErrorMessage == "" {
		result.Status = statusFailed
		result.ErrorMessage = waitErr.Error()
		finishRunResult(&result, startedAt, stderrCh, cg)
		return result
	}

	if !receivedEnd {
		result.Status = statusFailed
		if result.ErrorMessage == "" {
			result.ErrorMessage = "process finished without END"
		}
		finishRunResult(&result, startedAt, stderrCh, cg)
		return result
	}

	if strings.EqualFold(result.FinishReason, "error") {
		result.Status = statusFailed
		if result.ErrorMessage == "" {
			result.ErrorMessage = "model finished with error reason"
		}
	} else {
		result.Status = statusFinished
	}

	finishRunResult(&result, startedAt, stderrCh, cg)
	return result
}

func writeModelConfig(w *bufio.Writer, cfg ModelRunConfig) error {
	for _, v := range cfg.SetVars {
		if _, err := fmt.Fprintf(w, "SET %s %s\n", v.Name, v.Value); err != nil {
			return err
		}
	}

	if cfg.StopRule != nil {
		if _, err := fmt.Fprintf(w, "RULE STOP %s %s %s\n", cfg.StopRule.Left, cfg.StopRule.Op, cfg.StopRule.Right); err != nil {
			return err
		}
	}

	if len(cfg.Metrics) > 0 {
		if _, err := fmt.Fprintf(w, "METRIC %s\n", strings.Join(cfg.Metrics, ";")); err != nil {
			return err
		}
	}

	if cfg.MetricStep > 0 {
		rawStep := strconv.FormatFloat(cfg.MetricStep, 'g', -1, 64)
		if _, err := fmt.Fprintf(w, "METRIC_STEP %s\n", rawStep); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintln(w, "START")
	return err
}

func waitReady(scanner *bufio.Scanner, fail func(string)) bool {
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if line == "READY" {
			return true
		}

		if strings.HasPrefix(line, "ERR ") {
			fail("model returned " + line)
			return false
		}

		fail("unexpected line before READY: " + line)
		return false
	}

	if err := scanner.Err(); err != nil {
		fail("read READY: " + err.Error())
	} else {
		fail("process closed stdout before READY")
	}

	return false
}

func waitProcess(cmd *exec.Cmd, result *ModelRunResult) error {
	err := cmd.Wait()

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	return err
}

func finishRunResult(result *ModelRunResult, startedAt time.Time, stderrCh <-chan string, cg *ExperimentCgroup) {
	finishedAt := time.Now()
	result.FinishedAtUnixMS = finishedAt.UnixMilli()
	result.DurationMS = finishedAt.Sub(startedAt).Milliseconds()

	if cg != nil {
		result.PeakMemoryBytes = cg.PeakMemoryBytes()

		if cg.WasOOMKilled() && result.Status != statusCanceled && result.FinishReason != "timeout" {
			result.Status = statusFailed
			result.FinishReason = "memory_limit_exceeded"

			if result.ErrorMessage == "" {
				result.ErrorMessage = "model process exceeded cgroup memory limit"
			}
		}
	}

	select {
	case tail := <-stderrCh:
		result.StderrTail = tail
	case <-time.After(2 * time.Second):
		result.StderrTail = ""
	}
}

func captureLimited(r io.Reader, limit int) <-chan string {
	ch := make(chan string, 1)

	go func() {
		defer close(ch)

		buf := make([]byte, 0, min(limit, 4096))
		tmp := make([]byte, 4096)

		for {
			n, err := r.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
				if len(buf) > limit {
					buf = append([]byte(nil), buf[len(buf)-limit:]...)
				}
			}
			if err != nil {
				break
			}
		}

		ch <- string(buf)
	}()

	return ch
}
