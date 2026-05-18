package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
)

type CSVTraceWriter struct {
	file    *os.File
	writer  *csv.Writer
	metrics []string
	rows    uint64
}

func NewCSVTraceWriter(filePath string, metrics []string) (*CSVTraceWriter, error) {
	if len(metrics) == 0 {
		return nil, fmt.Errorf("empty metric list")
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return nil, err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}

	w := csv.NewWriter(file)
	if err := w.Write(metrics); err != nil {
		_ = file.Close()
		return nil, err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		_ = file.Close()
		return nil, err
	}

	return &CSVTraceWriter{
		file:    file,
		writer:  w,
		metrics: metrics,
	}, nil
}

func (w *CSVTraceWriter) WriteStat(stat map[string]string) error {
	row := make([]string, len(w.metrics))
	for i, metric := range w.metrics {
		row[i] = stat[metric]
	}

	if err := w.writer.Write(row); err != nil {
		return err
	}

	w.writer.Flush()
	if err := w.writer.Error(); err != nil {
		return err
	}

	w.rows++
	return nil
}

func (w *CSVTraceWriter) Close() error {
	if w == nil {
		return nil
	}

	w.writer.Flush()
	writerErr := w.writer.Error()
	closeErr := w.file.Close()

	if writerErr != nil {
		return writerErr
	}
	return closeErr
}

func (w *CSVTraceWriter) Rows() uint64 {
	if w == nil {
		return 0
	}
	return w.rows
}

func (w *CSVTraceWriter) Path() string {
	if w == nil || w.file == nil {
		return ""
	}
	return w.file.Name()
}
