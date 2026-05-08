package generator

import (
	"strconv"
)

func Parse(data []byte) (*Model, error) {
	model := NewModel()
	scanner := NewLineScanner(data)

	if err := parseHash(scanner, model); err != nil {
		return nil, err
	}

	entityCount, err := parseSectionHeader(scanner, "ENTITY")
	if err != nil {
		return nil, err
	}
	model.Entities = make([]*EntityDesc, 0, entityCount)
	if err := parseEntitySection(scanner, model, entityCount); err != nil {
		return nil, err
	}

	varCount, err := parseSectionHeader(scanner, "VAR")
	if err != nil {
		return nil, err
	}
	model.Vars = make([]*VarDesc, 0, varCount)
	if err := parseVarSection(scanner, model, varCount); err != nil {
		return nil, err
	}

	resourceCount, err := parseSectionHeader(scanner, "RESOURCE")
	if err != nil {
		return nil, err
	}
	model.Resources = make([]*ResourceDesc, 0, resourceCount)
	if err := parseResourceSection(scanner, model, resourceCount); err != nil {
		return nil, err
	}

	blockCount, err := parseSectionHeader(scanner, "BLOCK")
	if err != nil {
		return nil, err
	}
	model.Blocks = make([]*BlockDesc, 0, blockCount)
	// parseBlockSection будет следующим шагом

	return model, nil
}

func parseHash(scanner *LineScanner, model *Model) error {
	if !scanner.ScanNoEmpty() {
		return parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected SHA-256 hash`)
	}

	hash := scanner.Word()

	if extra := scanner.Word(); extra != nil {
		return parseErr(scanner.LineNum(), ErrIncorrectHash, `expected single SHA-256 hash token`)
	}

	if len(hash) != 64 {
		return parseErr(scanner.LineNum(), ErrIncorrectHash, `expected 64 hex characters, got `+strconv.Itoa(len(hash)))
	}

	if !isHex(hash) {
		return parseErr(scanner.LineNum(), ErrIncorrectHash, `expected hexadecimal SHA-256 hash`)
	}

	model.Hash = string(hash)
	return nil
}

func parseSectionHeader(scanner *LineScanner, expectedName string) (int, error) {
	if !scanner.ScanNoEmpty() {
		return 0, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected section "`+expectedName+` <count> {"`)
	}

	name := scanner.Word()
	if string(name) != expectedName {
		return 0, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected "`+expectedName+`", got "`+string(name)+`"`)
	}

	countToken := scanner.Word()
	if countToken == nil {
		return 0, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected count after "`+expectedName+`"`)
	}

	count, ok := parseNonNegativeInt(countToken)
	if !ok {
		return 0, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected non-negative count, got "`+string(countToken)+`"`)
	}

	err := expectOpeningBrace(scanner, "section count")
	if err != nil {
		return 0, err
	}

	return count, nil
}

func parseNonNegativeInt(data []byte) (int, bool) {
	if len(data) == 0 {
		return 0, false
	}

	for _, c := range data {
		if c < '0' || c > '9' {
			return 0, false
		}
	}

	n, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, false
	}

	return n, true
}

func isHex(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	for _, c := range data {
		if c >= '0' && c <= '9' {
			continue
		}
		if c >= 'a' && c <= 'f' {
			continue
		}
		if c >= 'A' && c <= 'F' {
			continue
		}
		return false
	}

	return true
}
