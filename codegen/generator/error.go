package generator

import (
	"errors"
	"strconv"
)

var (
	ErrUnexpectedEOF         = errors.New("unexpected end of file")
	ErrIncorrectHash         = errors.New("incorrect hash")
	ErrIncorrectFormat       = errors.New("incorrect format")
	ErrInvalidIdentifier     = errors.New("invalid identifier")
	ErrDuplicateName         = errors.New("duplicate name")
	ErrUnknownType           = errors.New("unknown type")
	ErrUnknownBlockType      = errors.New("unknown block type")
	ErrUnknownBlockParam     = errors.New("unknown block parameter")
	ErrUnknownBlockingPolicy = errors.New("unknown blocking policy")
	ErrUnknownReference      = errors.New("unknown reference")
	ErrInvalidSectionCount   = errors.New("invalid section count")
)

type ParseErr struct {
	Line int
	Err  error
	Msg  string
}

func parseErr(line int, err error, msg string) error {
	if err == nil {
		return nil
	}
	return &ParseErr{Line: line, Err: err, Msg: msg}
}

func (e *ParseErr) Error() string {
	if e.Msg == "" {
		return "line " + strconv.Itoa(e.Line) + ": " + e.Err.Error()
	}
	return "line " + strconv.Itoa(e.Line) + ": " + e.Err.Error() + ": " + e.Msg
}

func (e *ParseErr) Unwrap() error {
	return e.Err
}
