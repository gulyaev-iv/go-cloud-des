package generator

import (
	"bytes"
)

type LineScanner struct {
	data    []byte
	line    []byte
	lineNum int
}

func NewLineScanner(data []byte) *LineScanner {
	return &LineScanner{
		data: data,
	}
}

func (s *LineScanner) Scan() bool {
	if len(s.data) == 0 {
		s.line = nil
		return false
	}

	idx := bytes.IndexByte(s.data, '\n')
	if idx == -1 {
		s.line = s.data
		s.data = nil
		s.lineNum++
		return true
	}

	s.line = s.data[:idx]
	s.data = s.data[idx+1:]
	s.lineNum++

	if len(s.line) > 0 && s.line[len(s.line)-1] == '\r' {
		s.line = s.line[:len(s.line)-1]
	}

	return true
}

func (s *LineScanner) ScanNoEmpty() bool {
	for s.Scan() {
		if len(bytes.TrimSpace(s.line)) != 0 {
			return true
		}
	}
	return false
}

func (s *LineScanner) Word() []byte {
	s.line = trimLeftSpaces(s.line)
	if len(s.line) == 0 {
		return nil
	}

	i := 0
	for i < len(s.line) && !(s.line[i] == ' ' || s.line[i] == '\t') {
		i++
	}
	word := s.line[:i]
	s.line = s.line[i:]
	return word
}

func (s *LineScanner) Line() []byte {
	return trimLeftSpaces(s.line)
}

func trimLeftSpaces(b []byte) []byte {
	i := 0
	for i < len(b) && isSpace(b[i]) {
		i++
	}
	return b[i:]
}

func (s *LineScanner) LineNum() int {
	return s.lineNum
}
