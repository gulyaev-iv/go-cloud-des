//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultCgroupRoot   = "/sys/fs/cgroup"
	defaultCgroupParent = "go-cloud-des"
)

type ExperimentCgroupConfig struct {
	Root             string
	Parent           string
	ModelHash        string
	ExperimentID     string
	MemoryLimitBytes uint64
}

type ExperimentCgroup struct {
	path string
}

func NewExperimentCgroup(cfg ExperimentCgroupConfig) (*ExperimentCgroup, error) {
	if cfg.MemoryLimitBytes == 0 {
		return nil, fmt.Errorf("memory limit must be > 0")
	}

	root := strings.TrimSpace(cfg.Root)
	if root == "" {
		root = defaultCgroupRoot
	}
	root = filepath.Clean(root)

	parent := safeCgroupName(cfg.Parent)
	if parent == "" {
		parent = defaultCgroupParent
	}

	name := safeCgroupName(cfg.ModelHash + "-" + cfg.ExperimentID)
	if name == "" {
		return nil, fmt.Errorf("empty cgroup name")
	}

	if err := ensureCgroupV2Root(root); err != nil {
		return nil, err
	}

	if err := enableCgroupController(root, "memory"); err != nil {
		return nil, fmt.Errorf("enable memory controller on root cgroup: %w", err)
	}

	parentPath := filepath.Join(root, parent)
	if err := os.MkdirAll(parentPath, 0o755); err != nil {
		return nil, fmt.Errorf("create parent cgroup: %w", err)
	}

	if err := enableCgroupController(parentPath, "memory"); err != nil {
		return nil, fmt.Errorf("enable memory controller on parent cgroup: %w", err)
	}

	path := filepath.Join(parentPath, name)

	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("remove stale cgroup: %w", err)
	}

	if err := os.Mkdir(path, 0o755); err != nil {
		return nil, fmt.Errorf("create experiment cgroup: %w", err)
	}

	if err := os.WriteFile(
		filepath.Join(path, "memory.max"),
		[]byte(strconv.FormatUint(cfg.MemoryLimitBytes, 10)),
		0o644,
	); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("set memory.max: %w", err)
	}

	return &ExperimentCgroup{path: path}, nil
}

func ensureCgroupV2Root(root string) error {
	if _, err := os.Stat(filepath.Join(root, "cgroup.controllers")); err != nil {
		return fmt.Errorf("cgroups v2 root is not available at %s: %w", root, err)
	}

	if _, err := os.Stat(filepath.Join(root, "cgroup.subtree_control")); err != nil {
		return fmt.Errorf("cgroups v2 subtree control is not available at %s: %w", root, err)
	}

	return nil
}

func enableCgroupController(path string, controller string) error {
	controllersRaw, err := os.ReadFile(filepath.Join(path, "cgroup.controllers"))
	if err != nil {
		return fmt.Errorf("read cgroup.controllers: %w", err)
	}

	if !hasCgroupField(string(controllersRaw), controller) {
		return fmt.Errorf("controller %q is not available in %s", controller, path)
	}

	subtreeControlPath := filepath.Join(path, "cgroup.subtree_control")

	subtreeRaw, err := os.ReadFile(subtreeControlPath)
	if err != nil {
		return fmt.Errorf("read cgroup.subtree_control: %w", err)
	}

	if hasCgroupField(string(subtreeRaw), controller) {
		return nil
	}

	if err := os.WriteFile(subtreeControlPath, []byte("+"+controller), 0o644); err != nil {
		return fmt.Errorf("write cgroup.subtree_control: %w", err)
	}

	return nil
}

func hasCgroupField(raw string, name string) bool {
	for _, field := range strings.Fields(raw) {
		if field == name {
			return true
		}
	}
	return false
}

func (c *ExperimentCgroup) AddProcess(pid int) error {
	if c == nil {
		return nil
	}
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}

	return os.WriteFile(filepath.Join(c.path, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0o644)
}

func (c *ExperimentCgroup) PeakMemoryBytes() uint64 {
	if c == nil {
		return 0
	}

	for _, name := range []string{"memory.peak", "memory.current"} {
		raw, err := os.ReadFile(filepath.Join(c.path, name))
		if err != nil {
			continue
		}

		value, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
		if err == nil {
			return value
		}
	}

	return 0
}

func (c *ExperimentCgroup) WasOOMKilled() bool {
	if c == nil {
		return false
	}

	for _, name := range []string{"memory.events.local", "memory.events"} {
		raw, err := os.ReadFile(filepath.Join(c.path, name))
		if err != nil {
			continue
		}

		if cgroupEventValue(string(raw), "oom_kill") > 0 {
			return true
		}
	}

	return false
}

func cgroupEventValue(raw string, key string) uint64 {
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != key {
			continue
		}

		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			return value
		}
	}

	return 0
}

func (c *ExperimentCgroup) Close() error {
	if c == nil || c.path == "" {
		return nil
	}

	var lastErr error

	for i := 0; i < 20; i++ {
		err := os.Remove(c.path)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return nil
		}

		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("remove cgroup %s: %w", c.path, lastErr)
}

func safeCgroupName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		" ", "_",
		"\t", "_",
		"\n", "_",
		"\r", "_",
		"\x00", "_",
		"..", "_",
	)

	value = replacer.Replace(value)

	if len(value) > 200 {
		value = value[:200]
	}

	return value
}
