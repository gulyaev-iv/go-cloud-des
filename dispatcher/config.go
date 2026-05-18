package main

import (
	"strings"
	"time"
)

type Config struct {
	ExperimentID string
	ModelHash    string

	BinaryKey    string
	BinarySHA256 string

	SetFlags    stringList
	MetricsRaw  string
	StopRuleRaw string
	MetricStep  float64

	StoreMetrics bool

	WorkDir          string
	RunTimeout       time.Duration
	MemoryLimitBytes uint64

	S3Endpoint     string
	S3AccessKey    string
	S3SecretKey    string
	S3Bucket       string
	S3UseSSL       bool
	S3Region       string
	S3Prefix       string
	S3CreateBucket bool
}

type stringList []string

func (v *stringList) String() string {
	return strings.Join(*v, ",")
}

func (v *stringList) Set(value string) error {
	*v = append(*v, value)
	return nil
}
