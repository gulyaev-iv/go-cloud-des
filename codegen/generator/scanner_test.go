package generator

import (
	"testing"
)

func testNoNewline(text string, lines []string, t *testing.T) {
	scanner := NewLineScanner([]byte(text))

	lineNum := 0
	for ; scanner.Scan(); lineNum++ {
		if string(scanner.Line()) != lines[lineNum] {
			t.Errorf("bad scan: line %v\n\texpected: %q\n\t     got: %q", lineNum, lines[lineNum], string(scanner.Line()))
		}
	}

	if lineNum != len(lines) {
		t.Errorf("bad scan: expected %v lines, got %v lines", len(lines), lineNum)
	}
}

func TestScanLineEmptyFinalLine(t *testing.T) {
	const text = "abcdefghijklmn\nopqrstuvwxyz\n\n"
	lines := []string{
		"abcdefghijklmn",
		"opqrstuvwxyz",
		"",
	}

	testNoNewline(text, lines, t)

}
func TestScanLineNoNewline(t *testing.T) {
	const text = "  abcdefgh ijklmn \nopqrstuvwxyz"
	lines := []string{
		"abcdefgh ijklmn ",
		"opqrstuvwxyz",
	}

	testNoNewline(text, lines, t)
}

func TestScanLineWithCRLF(t *testing.T) {
	const text = "abcdefghijklmn\r\nopqrstuvwxyz"
	lines := []string{
		"abcdefghijklmn",
		"opqrstuvwxyz",
	}

	testNoNewline(text, lines, t)
}

func testNoEmptyLine(text string, lines []string, t *testing.T) {
	scanner := NewLineScanner([]byte(text))

	lineNum := 0
	for ; scanner.ScanNoEmpty(); lineNum++ {
		if string(scanner.Line()) != lines[lineNum] {
			t.Errorf("bad scan: line %v\n\texpected: %q\n\t     got: %q", lineNum, lines[lineNum], string(scanner.Line()))
		}
	}

	if lineNum != len(lines) {
		t.Errorf("bad scan: expected %v lines, got %v lines", len(lines), lineNum)
	}
}

func TestScanNoEmpty(t *testing.T) {
	const text = "abcdefghijklmn\nopqrstuvwxyz\n\n      	\ndasdasdas"
	lines := []string{
		"abcdefghijklmn",
		"opqrstuvwxyz",
		"dasdasdas",
	}

	testNoEmptyLine(text, lines, t)
}

func TestWordWithSpace(t *testing.T) {
	const text = " asahfah lgm3249 _das 2		asfa"
	words := []string{
		"asahfah",
		"lgm3249",
		"_das",
		"2",
		"asfa",
	}

	scanner := NewLineScanner([]byte(text))

	scanner.Scan()
	wordNum := 0
	for word := scanner.Word(); word != nil; word = scanner.Word() {
		if string(word) != words[wordNum] {
			t.Errorf("bad scan: word %v\n\texpected: %q\n\t     got: %q", wordNum, words[wordNum], string(word))
		}
		wordNum++
	}

	if wordNum != len(words) {
		t.Errorf("bad scan: expected %v words, got %v words", len(words), wordNum)
	}
}

func TestCountLine(t *testing.T) {
	const text = "abcdefghijklmn\nopqrstuvwxyz\n\n      	\ndasdasdas"
	lineNum := 5

	scanner := NewLineScanner([]byte(text))
	for scanner.Scan() {
	}

	if scanner.LineNum() != lineNum {
		t.Errorf("bad counting line: expected %v lines, scanning %v lines", lineNum, scanner.LineNum())
	}
}
