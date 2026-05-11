package generator

type Result struct {
	Hash   string
	Source []byte
}

func Generate(data []byte) (*Result, error) {
	model, err := Parse(data)
	if err != nil {
		return nil, err
	}

	if err := ValidateAndLink(model); err != nil {
		return nil, err
	}

	source, err := emit(model)
	if err != nil {
		return nil, err
	}

	return &Result{
		Hash:   model.Hash,
		Source: source,
	}, nil
}
