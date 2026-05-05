package generator

import (
	"bytes"
)

type LineScanner struct {
	data []byte
}

func NewLineScanner(data []byte) *LineScanner {
	return &LineScanner{
		data: data,
	}
}

func (s *LineScanner) Next() ([]byte, bool) {
	if len(s.data) == 0 {
		return nil, false
	}

	idx := bytes.IndexByte(s.data, '\n')

	if idx == -1 {
		line := s.data
		s.data = nil
		return line, true
	}

	line := s.data[:idx]
	s.data = s.data[idx+1:]

	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}

	return line, true
}
