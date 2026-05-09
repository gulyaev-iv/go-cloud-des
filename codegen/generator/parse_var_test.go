package generator

import (
	"errors"
	"testing"
)

func TestParseVarWithDefault(t *testing.T) {
	input := []byte(`arrivalRate float64 1.25`)
	testParseVar(t, input, "arrivalRate", ValueFloat64, "1.25")
}

func TestParseVarWithoutDefault(t *testing.T) {
	input := []byte(`rejected uint64`)
	testParseVar(t, input, "rejected", ValueUint64, "0")
}

func testParseVar(t *testing.T, input []byte, expectName string, expectType ValueType, expectValue string) {
	t.Helper()

	scanner := NewLineScanner(input)

	varDesc, err := parseVar(scanner)
	if err != nil {
		t.Fatalf("parseVar() error = %v", err)
	}

	if varDesc.Name != expectName {
		t.Fatalf("varDesc.Name = %q, want %q", varDesc.Name, expectName)
	}

	if varDesc.Type != expectType {
		t.Fatalf("varDesc.Type = %v, want %v", varDesc.Type, expectType)
	}

	if varDesc.Default != expectValue {
		t.Fatalf("varDesc.Default = %q, want %q", varDesc.Default, expectValue)
	}
}

func TestParseVarUnexpectedEOF(t *testing.T) {
	scanner := NewLineScanner(nil)

	_, err := parseVar(scanner)
	if !errors.Is(err, ErrUnexpectedEOF) {
		t.Fatalf("parseVar() error = %v, want ErrUnexpectedEOF", err)
	}
}

func TestParseVarUnknownType(t *testing.T) {
	input := []byte(`name string`)

	scanner := NewLineScanner(input)

	_, err := parseVar(scanner)
	if !errors.Is(err, ErrUnknownType) {
		t.Fatalf("parseVar() error = %v, want ErrUnknownType", err)
	}
}

func TestParseVarInvalidDefaultValue(t *testing.T) {
	input := []byte(`enabled bool yes`)

	scanner := NewLineScanner(input)

	_, err := parseVar(scanner)
	if !errors.Is(err, ErrIncorrectFormat) {
		t.Fatalf("parseVar() error = %v, want ErrIncorrectFormat", err)
	}
}

func TestParseVarUnexpectedTokenAfterDefault(t *testing.T) {
	input := []byte(`arrivalRate float64 1.25 extra`)

	scanner := NewLineScanner(input)

	_, err := parseVar(scanner)
	if !errors.Is(err, ErrIncorrectFormat) {
		t.Fatalf("parseVar() error = %v, want ErrIncorrectFormat", err)
	}
}

func TestParseVarInvalidName(t *testing.T) {
	input := []byte(`1arrivalRate float64 1.25`)

	scanner := NewLineScanner(input)

	_, err := parseVar(scanner)
	if !errors.Is(err, ErrInvalidIdentifier) {
		t.Fatalf("parseVar() error = %v, want ErrInvalidIdentifier", err)
	}
}

func TestParseVarReservedKeywordName(t *testing.T) {
	input := []byte(`ENTITY float64 1.25`)

	scanner := NewLineScanner(input)

	_, err := parseVar(scanner)
	if !errors.Is(err, ErrInvalidIdentifier) {
		t.Fatalf("parseVar() error = %v, want ErrInvalidIdentifier", err)
	}
}

func TestParseVarSection(t *testing.T) {
	input := []byte(`arrivalRate float64 1.25
		serviceRate float64
		rejected uint64 0
		}`)

	scanner := NewLineScanner(input)
	model := NewModel()
	model.Vars = make([]*VarDesc, 0, 3)

	err := parseVarSection(scanner, model, 3)
	if err != nil {
		t.Fatalf("parseVarSection() error = %v", err)
	}

	if len(model.Vars) != 3 {
		t.Fatalf("len(model.Vars) = %d, want 3", len(model.Vars))
	}

	testVar(t, model, 0, "arrivalRate", ValueFloat64, "1.25")
	testVar(t, model, 1, "serviceRate", ValueFloat64, "0.0")
	testVar(t, model, 2, "rejected", ValueUint64, "0")
}

func testVar(t *testing.T, model *Model, index int, name string, valueType ValueType, defaultValue string) {
	t.Helper()

	if model.Vars[index].Name != name {
		t.Fatalf("model.Vars[%d].Name = %q, want %q", index, model.Vars[index].Name, name)
	}

	if model.Vars[index].Type != valueType {
		t.Fatalf("%s.Type = %v, want %v", name, model.Vars[index].Type, valueType)
	}

	if model.Vars[index].Default != defaultValue {
		t.Fatalf("%s.Default = %q, want %q", name, model.Vars[index].Default, defaultValue)
	}

	if model.VarByName[name] != model.Vars[index] {
		t.Fatalf("model.VarByName[%q] does not point to model.Vars[%d]", name, index)
	}
}

func TestParseVarSectionDuplicateGlobalName(t *testing.T) {
	input := []byte(`arrivalRate float64 1.25
		}`)

	scanner := NewLineScanner(input)
	model := NewModel()
	model.EntityByName["arrivalRate"] = &EntityDesc{Name: "arrivalRate"}
	model.Vars = make([]*VarDesc, 0, 1)

	err := parseVarSection(scanner, model, 1)
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("parseVarSection() error = %v, want ErrDuplicateName", err)
	}
}

func TestParseVarSectionMissingClosingBrace(t *testing.T) {
	input := []byte(`arrivalRate float64 1.25`)

	scanner := NewLineScanner(input)
	model := NewModel()
	model.Vars = make([]*VarDesc, 0, 1)

	err := parseVarSection(scanner, model, 1)
	if !errors.Is(err, ErrUnexpectedEOF) {
		t.Fatalf("parseVarSection() error = %v, want ErrUnexpectedEOF", err)
	}
}

func TestParseVarSectionTooManyObjects(t *testing.T) {
	input := []byte(`arrivalRate float64 1.25
		serviceRate float64
		}`)

	scanner := NewLineScanner(input)
	model := NewModel()
	model.Vars = make([]*VarDesc, 0, 1)

	err := parseVarSection(scanner, model, 1)
	if !errors.Is(err, ErrIncorrectFormat) {
		t.Fatalf("parseVarSection() error = %v, want ErrIncorrectFormat", err)
	}
}
