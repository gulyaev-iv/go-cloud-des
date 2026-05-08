package generator

import (
	"bytes"
	"testing"
)

func BenchmarkBytesEqual(b *testing.B) {
	words := [][]byte{[]byte("{"), []byte("{as"), []byte("}"), nil}
	for _, w := range words {
		b.Run("bytesEqual", func(b *testing.B) {
			for b.Loop() {
				bytesEqual(w)
			}
		})
		b.Run("orBoolEqual", func(b *testing.B) {
			for b.Loop() {
				orBoolEqual(w)
			}
		})
	}
}

func bytesEqual(data []byte) bool {
	return bytes.Equal(data, []byte("{"))
}

func orBoolEqual(data []byte) bool {
	return string(data) == string([]byte("{"))
}

func TestBytesEqual(t *testing.T) {
	words := [][]byte{[]byte("{"), []byte("{as"), []byte("}"), nil}
	cor := []bool{true, false, false, false}
	for i, w := range words {
		if bytesEqual(w) != cor[i] {
			t.Errorf("incorrect equal: expecte \"{\", got %q", string(w))
		}
	}
}

func TestOrBoolEqual(t *testing.T) {
	words := [][]byte{[]byte("{"), []byte("{as"), []byte("}"), nil}
	cor := []bool{true, false, false, false}
	for i, w := range words {
		if orBoolEqual(w) != cor[i] {
			t.Errorf("incorrect equal: expecte \"{\", got %q", string(w))
		}
	}
}

func BenchmarkParseNonNegativeInt(b *testing.B) {
	a := []string{"12", "12314", "-123", "asdas", "0asda"}
	for _, w := range a {
		b.Run(w, func(b *testing.B) {
			for b.Loop() {

				parseNonNegativeInt([]byte(w))
			}
		})
	}
}
