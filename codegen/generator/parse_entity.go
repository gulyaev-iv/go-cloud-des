package generator

import "bytes"

func parseEntitySection(scanner *LineScanner, model *Model, count int) error {
	entities, err := parseSection(scanner, model, count, model.Entities, model.EntityByName, parseEntity, func(e *EntityDesc) string { return e.Name })
	if err != nil {
		return err
	}

	model.Entities = entities
	return nil
}

func parseEntity(scanner *LineScanner) (*EntityDesc, error) {
	if !scanner.ScanNoEmpty() {
		return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected entity declaration`)
	}

	nameToken := scanner.Word()
	name, err := validateName(scanner, nameToken)
	if err != nil {
		return nil, err
	}

	err = expectOpeningBrace(scanner, "entity name")
	if err != nil {
		return nil, err
	}

	entity := &EntityDesc{
		Name:        name,
		Fields:      make([]*FieldDesc, 0),
		FieldByName: make(map[string]*FieldDesc),
	}

	for scanner.ScanNoEmpty() {
		word := scanner.Word()
		if bytes.Equal(word, []byte("}")) {
			if extra := scanner.Word(); extra != nil {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `unexpected token "`+string(extra)+`" after "}"`)
			}
			return entity, nil
		}

		field, err := parseEntityField(scanner, word)
		if err != nil {
			return nil, err
		}

		if field.Name == "id" {
			return nil, parseErr(scanner.LineNum(), ErrInvalidIdentifier, `field "id" is reserved`)
		}

		if _, ok := entity.FieldByName[field.Name]; ok {
			return nil, parseErr(scanner.LineNum(), ErrDuplicateName, `field "`+field.Name+`" already declared in entity "`+entity.Name+`"`)
		}

		entity.Fields = append(entity.Fields, field)
		entity.FieldByName[field.Name] = field
	}

	return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected "}" after entity declaration`)
}

func parseEntityField(scanner *LineScanner, firstToken []byte) (*FieldDesc, error) {
	spec, err := parseTypedValueSpec(scanner, firstToken, "field")
	if err != nil {
		return nil, err
	}

	return &FieldDesc{
		Name:    spec.Name,
		Type:    spec.Type,
		Default: spec.Default,
	}, nil
}
