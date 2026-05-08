package generator

func parseVarSection(scanner *LineScanner, model *Model, count int) error {
	vars, err := parseSection(scanner, model, count, model.Vars, model.VarByName, parseVar, func(v *VarDesc) string { return v.Name })
	if err != nil {
		return err
	}

	model.Vars = vars
	return nil
}

func parseVar(scanner *LineScanner) (*VarDesc, error) {
	if !scanner.ScanNoEmpty() {
		return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected variable declaration`)
	}

	firstToken := scanner.Word()
	spec, err := parseTypedValueSpec(scanner, firstToken, "variable")
	if err != nil {
		return nil, err
	}

	return &VarDesc{
		Name:    spec.Name,
		Type:    spec.Type,
		Default: spec.Default,
	}, nil
}
