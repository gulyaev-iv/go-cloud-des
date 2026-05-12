package generator

import (
	"bytes"
	"strconv"
)

type ParseFunc[T any] func(scanner *LineScanner) (*T, error)

func parseSection[T any](scanner *LineScanner, model *Model, count int, objects []*T, byName map[string]*T, parse ParseFunc[T], getName func(*T) string) ([]*T, error) {
	for i := 0; i < count; i++ {
		obj, err := parse(scanner)
		if err != nil {
			return nil, err
		}

		name := getName(obj)
		if err := registerGlobalName(model, scanner, name); err != nil {
			return nil, err
		}

		objects = append(objects, obj)
		byName[name] = obj
	}

	if err := expectClosingBrace(scanner); err != nil {
		return nil, err
	}

	return objects, nil
}

func parseType(token []byte) (ValueType, bool) {
	switch string(token) {
	case "float64":
		return ValueFloat64, true
	case "int64":
		return ValueInt64, true
	case "uint64":
		return ValueUint64, true
	case "bool":
		return ValueBool, true
	default:
		return 0, false
	}
}

func defaultValueForType(t ValueType) string {
	switch t {
	case ValueFloat64:
		return "0.0"
	case ValueInt64:
		return "0"
	case ValueUint64:
		return "0"
	case ValueBool:
		return "false"
	default:
		return ""
	}
}

func expectOpeningBrace(scanner *LineScanner, expected string) error {
	openBrace := scanner.Word()
	if !bytes.Equal(openBrace, []byte("{")) {
		return parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected "{" after `+expected)
	}

	if extra := scanner.Word(); extra != nil {
		return parseErr(scanner.LineNum(), ErrIncorrectFormat, `unexpected token "`+string(extra)+`" after "{"`)
	}

	return nil
}

func expectClosingBrace(scanner *LineScanner) error {
	if !scanner.ScanNoEmpty() {
		return parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected "}"`)
	}

	closeBrace := scanner.Word()
	if !bytes.Equal(closeBrace, []byte("}")) {
		return parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected "}", got "`+string(closeBrace)+`"`)
	}

	if extra := scanner.Word(); extra != nil {
		return parseErr(scanner.LineNum(), ErrIncorrectFormat, `unexpected token "`+string(extra)+`" after "}"`)
	}

	return nil
}

func registerGlobalName(model *Model, scanner *LineScanner, name string) error {
	if _, ok := model.EntityByName[name]; ok {
		return parseErr(scanner.LineNum(), ErrDuplicateName, `name "`+name+`" already declared`)
	}
	if _, ok := model.VarByName[name]; ok {
		return parseErr(scanner.LineNum(), ErrDuplicateName, `name "`+name+`" already declared`)
	}
	if _, ok := model.ResourceByName[name]; ok {
		return parseErr(scanner.LineNum(), ErrDuplicateName, `name "`+name+`" already declared`)
	}
	if _, ok := model.BlockByName[name]; ok {
		return parseErr(scanner.LineNum(), ErrDuplicateName, `name "`+name+`" already declared`)
	}

	return nil
}

func isValidLiteralForType(value string, t ValueType) bool {
	switch t {
	case ValueFloat64:
		_, err := strconv.ParseFloat(value, 64)
		return err == nil
	case ValueInt64:
		_, err := strconv.ParseInt(value, 10, 64)
		return err == nil
	case ValueUint64:
		_, err := strconv.ParseUint(value, 10, 64)
		return err == nil
	case ValueBool:
		return value == "true" || value == "false"
	default:
		return false
	}
}

func isIdentifier(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	if !isIdentStart(data[0]) {
		return false
	}

	for i := 1; i < len(data); i++ {
		if !isIdentPart(data[i]) {
			return false
		}
	}

	return true
}

func isIdentStart(c byte) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		c == '_'
}

func isDSLKeyword(name string) bool {
	switch name {
	case "ENTITY", "VAR", "RESOURCE", "BLOCK",
		"CREATE", "QUEUE", "DELAY", "SEIZE", "RELEASE", "ASSIGN", "BRANCH", "TERMINATE",
		"set", "if", "else", "next":
		return true
	default:
		return false
	}
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func validateName(scanner *LineScanner, token []byte) (string, error) {
	if !isIdentifier(token) {
		return "", parseErr(scanner.LineNum(), ErrInvalidIdentifier, `invalid identifier "`+string(token)+`"`)
	}

	name := string(token)
	if isDSLKeyword(name) {
		return "", parseErr(scanner.LineNum(), ErrInvalidIdentifier, `reserved DSL keyword "`+name+`"`)
	}

	return name, nil
}

func parseDefaultValue(scanner *LineScanner, t ValueType) (string, error) {
	token := scanner.Word()
	if token == nil {
		return defaultValueForType(t), nil
	}

	if extra := scanner.Word(); extra != nil {
		return "", parseErr(scanner.LineNum(), ErrIncorrectFormat, `unexpected token "`+string(extra)+`" after default value`)
	}

	value := string(token)

	if !isValidLiteralForType(value, t) {
		return "", parseErr(scanner.LineNum(), ErrIncorrectFormat, `invalid default value "`+value+`"`)
	}

	return value, nil
}

type typedValueSpec struct {
	Name    string
	Type    ValueType
	Default string
}

func parseTypedValueSpec(scanner *LineScanner, firstToken []byte, kind string) (typedValueSpec, error) {
	name, err := validateName(scanner, firstToken)
	if err != nil {
		return typedValueSpec{}, err
	}

	typeToken := scanner.Word()
	if typeToken == nil {
		return typedValueSpec{}, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected `+kind+` type`)
	}

	valueType, ok := parseType(typeToken)
	if !ok {
		return typedValueSpec{}, parseErr(scanner.LineNum(), ErrUnknownType, `unknown `+kind+` type "`+string(typeToken)+`"`)
	}

	defaultValue, err := parseDefaultValue(scanner, valueType)
	if err != nil {
		return typedValueSpec{}, err
	}

	return typedValueSpec{
		Name:    name,
		Type:    valueType,
		Default: defaultValue,
	}, nil
}
