package generator

import (
	"errors"
	"testing"
)

func newScannedLineScanner(line string) *LineScanner {
	scanner := NewLineScanner([]byte(line))
	scanner.Scan()
	return scanner
}

func TestIsIdentifier(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{in: "Order", want: true},
		{in: "_Order", want: true},
		{in: "Order_1", want: true},
		{in: "order123", want: true},

		{in: "", want: false},
		{in: "1Order", want: false},
		{in: "Order.Name", want: false},
		{in: "Order-Name", want: false},
		{in: "Order Name", want: false},
		{in: "Заказ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := isIdentifier([]byte(tt.in))
			if got != tt.want {
				t.Fatalf("isIdentifier(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		scanner := newScannedLineScanner("Order")
		name, err := validateName(scanner, scanner.Word())
		if err != nil {
			t.Fatalf("validateName() error = %v", err)
		}
		if name != "Order" {
			t.Fatalf("name = %q, want %q", name, "Order")
		}
	})

	t.Run("invalid identifier", func(t *testing.T) {
		scanner := newScannedLineScanner("1Order")
		_, err := validateName(scanner, scanner.Word())
		if !errors.Is(err, ErrInvalidIdentifier) {
			t.Fatalf("error = %v, want ErrInvalidIdentifier", err)
		}
	})

	t.Run("dsl keyword", func(t *testing.T) {
		scanner := newScannedLineScanner("ENTITY")
		_, err := validateName(scanner, scanner.Word())
		if !errors.Is(err, ErrInvalidIdentifier) {
			t.Fatalf("error = %v, want ErrInvalidIdentifier", err)
		}
	})
}

func TestParseDefaultValue(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		valueT  ValueType
		want    string
		wantErr error
	}{
		{name: "empty float64", line: "", valueT: ValueFloat64, want: "0.0"},
		{name: "empty int64", line: "", valueT: ValueInt64, want: "0"},
		{name: "empty uint64", line: "", valueT: ValueUint64, want: "0"},
		{name: "empty bool", line: "", valueT: ValueBool, want: "false"},

		{name: "valid float64", line: "1.25", valueT: ValueFloat64, want: "1.25"},
		{name: "valid int64", line: "-10", valueT: ValueInt64, want: "-10"},
		{name: "valid uint64", line: "10", valueT: ValueUint64, want: "10"},
		{name: "valid bool true", line: "true", valueT: ValueBool, want: "true"},
		{name: "valid bool false", line: "false", valueT: ValueBool, want: "false"},

		{name: "invalid uint64", line: "-1", valueT: ValueUint64, wantErr: ErrIncorrectFormat},
		{name: "invalid bool", line: "yes", valueT: ValueBool, wantErr: ErrIncorrectFormat},
		{name: "extra token", line: "1 2", valueT: ValueInt64, wantErr: ErrIncorrectFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := newScannedLineScanner(tt.line)
			got, err := parseDefaultValue(scanner, tt.valueT)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseDefaultValue() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("value = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTypedValueSpec(t *testing.T) {
	t.Run("with default", func(t *testing.T) {
		scanner := newScannedLineScanner("arrivalRate float64 1.25")
		spec, err := parseTypedValueSpec(scanner, scanner.Word(), "variable")
		if err != nil {
			t.Fatalf("parseTypedValueSpec() error = %v", err)
		}

		if spec.Name != "arrivalRate" {
			t.Fatalf("name = %q, want %q", spec.Name, "arrivalRate")
		}
		if spec.Type != ValueFloat64 {
			t.Fatalf("type = %v, want %v", spec.Type, ValueFloat64)
		}
		if spec.Default != "1.25" {
			t.Fatalf("default = %q, want %q", spec.Default, "1.25")
		}
	})

	t.Run("without default", func(t *testing.T) {
		scanner := newScannedLineScanner("rejected uint64")
		spec, err := parseTypedValueSpec(scanner, scanner.Word(), "variable")
		if err != nil {
			t.Fatalf("parseTypedValueSpec() error = %v", err)
		}

		if spec.Name != "rejected" {
			t.Fatalf("name = %q, want %q", spec.Name, "rejected")
		}
		if spec.Type != ValueUint64 {
			t.Fatalf("type = %v, want %v", spec.Type, ValueUint64)
		}
		if spec.Default != "0" {
			t.Fatalf("default = %q, want %q", spec.Default, "0")
		}
	})

	t.Run("unknown type", func(t *testing.T) {
		scanner := newScannedLineScanner("fieldName string")
		_, err := parseTypedValueSpec(scanner, scanner.Word(), "field")
		if !errors.Is(err, ErrUnknownType) {
			t.Fatalf("error = %v, want ErrUnknownType", err)
		}
	})
}

func TestExpectOpeningBrace(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		scanner := newScannedLineScanner("{")
		if err := expectOpeningBrace(scanner, "entity name"); err != nil {
			t.Fatalf("expectOpeningBrace() error = %v", err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		scanner := newScannedLineScanner("Order")
		err := expectOpeningBrace(scanner, "entity name")
		if !errors.Is(err, ErrIncorrectFormat) {
			t.Fatalf("error = %v, want ErrIncorrectFormat", err)
		}
	})

	t.Run("extra token", func(t *testing.T) {
		scanner := newScannedLineScanner("{ extra")
		err := expectOpeningBrace(scanner, "entity name")
		if !errors.Is(err, ErrIncorrectFormat) {
			t.Fatalf("error = %v, want ErrIncorrectFormat", err)
		}
	})
}

func TestExpectClosingBrace(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		scanner := NewLineScanner([]byte("}"))
		if err := expectClosingBrace(scanner); err != nil {
			t.Fatalf("expectClosingBrace() error = %v", err)
		}
	})

	t.Run("skip empty lines", func(t *testing.T) {
		scanner := NewLineScanner([]byte("\n  \n}"))
		if err := expectClosingBrace(scanner); err != nil {
			t.Fatalf("expectClosingBrace() error = %v", err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		scanner := NewLineScanner([]byte("abc"))
		err := expectClosingBrace(scanner)
		if !errors.Is(err, ErrIncorrectFormat) {
			t.Fatalf("error = %v, want ErrIncorrectFormat", err)
		}
	})

	t.Run("eof", func(t *testing.T) {
		scanner := NewLineScanner(nil)
		err := expectClosingBrace(scanner)
		if !errors.Is(err, ErrUnexpectedEOF) {
			t.Fatalf("error = %v, want ErrUnexpectedEOF", err)
		}
	})
}

func TestRegisterGlobalName(t *testing.T) {
	t.Run("new name", func(t *testing.T) {
		model := NewModel()
		scanner := newScannedLineScanner("Order")

		if err := registerGlobalName(model, scanner, "Order"); err != nil {
			t.Fatalf("registerGlobalName() error = %v", err)
		}
	})

	t.Run("duplicate entity", func(t *testing.T) {
		model := NewModel()
		model.EntityByName["Order"] = &EntityDesc{Name: "Order"}

		scanner := newScannedLineScanner("Order")
		err := registerGlobalName(model, scanner, "Order")
		if !errors.Is(err, ErrDuplicateName) {
			t.Fatalf("error = %v, want ErrDuplicateName", err)
		}
	})

	t.Run("duplicate var", func(t *testing.T) {
		model := NewModel()
		model.VarByName["arrivalRate"] = &VarDesc{Name: "arrivalRate"}

		scanner := newScannedLineScanner("arrivalRate")
		err := registerGlobalName(model, scanner, "arrivalRate")
		if !errors.Is(err, ErrDuplicateName) {
			t.Fatalf("error = %v, want ErrDuplicateName", err)
		}
	})
}

type testSectionObject struct {
	Name string
}

func parseTestSectionObject(scanner *LineScanner) (*testSectionObject, error) {
	if !scanner.ScanNoEmpty() {
		return nil, parseErr(scanner.LineNum(), ErrUnexpectedEOF, `expected test object`)
	}
	nameToken := scanner.Word()
	return &testSectionObject{Name: string(nameToken)}, nil
}

func TestParseSection(t *testing.T) {
	scanner := NewLineScanner([]byte("First\nSecond\n}"))
	model := NewModel()

	objects := make([]*testSectionObject, 0, 2)
	byName := make(map[string]*testSectionObject)

	got, err := parseSection(scanner, model, 2, objects, byName, parseTestSectionObject, func(o *testSectionObject) string { return o.Name })
	if err != nil {
		t.Fatalf("parseSection() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(objects) = %d, want 2", len(got))
	}

	if got[0].Name != "First" || got[1].Name != "Second" {
		t.Fatalf("unexpected objects: %+v", got)
	}

	if byName["First"] != got[0] {
		t.Fatalf(`byName["First"] does not point to first object`)
	}

	if byName["Second"] != got[1] {
		t.Fatalf(`byName["Second"] does not point to second object`)
	}
}

func TestParseSectionDuplicateName(t *testing.T) {
	scanner := NewLineScanner([]byte("First\nSecond\n}"))
	model := NewModel()
	model.EntityByName["First"] = &EntityDesc{Name: "First"}

	objects := make([]*testSectionObject, 0, 2)
	byName := make(map[string]*testSectionObject)

	_, err := parseSection(scanner, model, 2, objects, byName, parseTestSectionObject, func(o *testSectionObject) string { return o.Name })
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("parseSection() error = %v, want ErrDuplicateName", err)
	}
}

func TestParseSectionMissingClosingBrace(t *testing.T) {
	scanner := NewLineScanner([]byte("First\nSecond"))
	model := NewModel()

	objects := make([]*testSectionObject, 0, 2)
	byName := make(map[string]*testSectionObject)

	_, err := parseSection(scanner, model, 2, objects, byName, parseTestSectionObject, func(o *testSectionObject) string { return o.Name })
	if !errors.Is(err, ErrUnexpectedEOF) {
		t.Fatalf("parseSection() error = %v, want ErrUnexpectedEOF", err)
	}
}

func TestParseSectionTooManyObjects(t *testing.T) {
	scanner := NewLineScanner([]byte("First\nSecond\nThird\n}"))
	model := NewModel()

	objects := make([]*testSectionObject, 0, 2)
	byName := make(map[string]*testSectionObject)

	_, err := parseSection(scanner, model, 2, objects, byName, parseTestSectionObject, func(o *testSectionObject) string { return o.Name })
	if !errors.Is(err, ErrIncorrectFormat) {
		t.Fatalf("parseSection() error = %v, want ErrIncorrectFormat", err)
	}
}
