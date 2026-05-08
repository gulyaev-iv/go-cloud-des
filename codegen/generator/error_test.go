package generator

import (
	"fmt"
	"testing"
)

var sinkString string

func BenchmarkError(b *testing.B) {
	lineN := 10

	b.Run("fmt.Errorf", func(b *testing.B) {
		for b.Loop() {
			sinkString = errorf(lineN)
		}
	})

	b.Run("LineError", func(b *testing.B) {
		for b.Loop() {
			sinkString = lineerr(lineN)
		}
	})
}

func errorf(line int) string {
	err := fmt.Errorf("line %v: incorrect hash: expected \"ENTITY <count> {\"", line)
	return err.Error()
}

func lineerr(line int) string {
	err := parseErr(line, ErrIncorrectHash, `expected "ENTITY <count> {"`)
	return err.Error()
}
