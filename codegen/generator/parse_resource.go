package generator

import "bytes"

func parseResourceSection(scanner *LineScanner, model *Model, count int) error {
	resources, err := parseSection(scanner, model, count, model.Resources, model.ResourceByName, parseResource, func(r *ResourceDesc) string { return r.Name })
	if err != nil {
		return err
	}

	model.Resources = resources
	return nil
}

func parseResource(scanner *LineScanner) (*ResourceDesc, error) {
	if !scanner.ScanNoEmpty() {
		return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected resource declaration`)
	}

	nameTok := scanner.Word()
	name, err := validateName(scanner, nameTok)
	if err != nil {
		return nil, err
	}

	err = expectOpeningBrace(scanner, "resource name")
	if err != nil {
		return nil, err
	}

	resource := &ResourceDesc{
		Name:       name,
		SeizeNames: make([]string, 0),
	}

	hasCapacity := false

	for scanner.ScanNoEmpty() {
		param := scanner.Word()

		if bytes.Equal(param, []byte("}")) {
			if extra := scanner.Word(); extra != nil {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `unexpected token "`+string(extra)+`" after "}"`)
			}
			if !hasCapacity {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required resource parameter "capacity"`)
			}
			if len(resource.SeizeNames) == 0 {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `missing required resource parameter "seize"`)
			}
			return resource, nil
		}

		switch string(param) {
		case "capacity":
			if hasCapacity {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `duplicate resource parameter "capacity"`)
			}

			value := scanner.Word()
			if value == nil {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected capacity value`)
			}

			if extra := scanner.Word(); extra != nil {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `unexpected token "`+string(extra)+`" after capacity value`)
			}

			resource.Capacity = Expr(string(value))
			hasCapacity = true

		case "seize":
			value := scanner.Word()
			if value == nil {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `expected SEIZE block name`)
			}

			if extra := scanner.Word(); extra != nil {
				return nil, parseErr(scanner.LineNum(), ErrIncorrectFormat, `unexpected token "`+string(extra)+`" after SEIZE block name`)
			}

			name, err := validateName(scanner, value)
			if err != nil {
				return nil, err
			}

			resource.SeizeNames = append(resource.SeizeNames, name)

		default:
			return nil, parseErr(scanner.LineNum(), ErrUnknownBlockParam, `unknown resource parameter "`+string(param)+`"`)
		}
	}

	return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected "}" after resource declaration`)
}
