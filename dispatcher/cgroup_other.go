//go:build !linux

package main

import "fmt"

type ExperimentCgroupConfig struct {
	Root             string
	Parent           string
	ModelHash        string
	ExperimentID     string
	MemoryLimitBytes uint64
}

type ExperimentCgroup struct{}

func NewExperimentCgroup(cfg ExperimentCgroupConfig) (*ExperimentCgroup, error) {
	return nil, fmt.Errorf("cgroups v2 are supported only on linux")
}

func (c *ExperimentCgroup) AddProcess(pid int) error {
	return nil
}

func (c *ExperimentCgroup) PeakMemoryBytes() uint64 {
	return 0
}

func (c *ExperimentCgroup) WasOOMKilled() bool {
	return false
}

func (c *ExperimentCgroup) Close() error {
	return nil
}
