package generator

import (
	"errors"
	"testing"
)

func TestParseEntity(t *testing.T) {
	input := []byte(`Order {
		arrivalTime float64
		priority int64 5
		processed bool true
		}`)

	scanner := NewLineScanner(input)

	entity, err := parseEntity(scanner)
	if err != nil {
		t.Fatalf("parseEntity() error = %v", err)
	}

	if entity.Name != "Order" {
		t.Fatalf("entity.Name = %q, want %q", entity.Name, "Order")
	}

	if len(entity.Fields) != 3 {
		t.Fatalf("len(entity.Fields) = %d, want 3", len(entity.Fields))
	}

	testEntityField(t, entity, "arrivalTime", ValueFloat64, "0.0")

	testEntityField(t, entity, "priority", ValueInt64, "5")

	testEntityField(t, entity, "processed", ValueBool, "true")
}

func testEntityField(t *testing.T, entity *EntityDesc, fieldName string, expect ValueType, defaultValue string) {
	t.Helper()

	field := entity.FieldByName[fieldName]
	if field == nil {
		t.Fatalf("field %q not found", fieldName)
	}
	if field.Type != expect {
		t.Fatalf("%s.Type = %v, want %v", fieldName, field.Type, expect)
	}
	if field.Default != defaultValue {
		t.Fatalf("%s.Default = %q, want %q", fieldName, field.Default, defaultValue)
	}
}

func TestParseEntityEmptyFields(t *testing.T) {
	input := []byte(`Order {
}`)

	scanner := NewLineScanner(input)

	entity, err := parseEntity(scanner)
	if err != nil {
		t.Fatalf("parseEntity() error = %v", err)
	}

	if entity.Name != "Order" {
		t.Fatalf("entity.Name = %q, want %q", entity.Name, "Order")
	}

	if len(entity.Fields) != 0 {
		t.Fatalf("len(entity.Fields) = %d, want 0", len(entity.Fields))
	}

	if len(entity.FieldByName) != 0 {
		t.Fatalf("len(entity.FieldByName) = %d, want 0", len(entity.FieldByName))
	}
}

func TestParseEntityDuplicateField(t *testing.T) {
	input := []byte(`Order {
		priority int64
		priority int64
		}`)

	scanner := NewLineScanner(input)

	_, err := parseEntity(scanner)
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("parseEntity() error = %v, want ErrDuplicateName", err)
	}
}

func TestParseEntityReservedIDField(t *testing.T) {
	input := []byte(`Order {
		id uint64
		}`)

	scanner := NewLineScanner(input)

	_, err := parseEntity(scanner)
	if !errors.Is(err, ErrInvalidIdentifier) {
		t.Fatalf("parseEntity() error = %v, want ErrInvalidIdentifier", err)
	}
}

func TestParseEntityUnknownFieldType(t *testing.T) {
	input := []byte(`Order {
		name string
		}`)

	scanner := NewLineScanner(input)

	_, err := parseEntity(scanner)
	if !errors.Is(err, ErrUnknownType) {
		t.Fatalf("parseEntity() error = %v, want ErrUnknownType", err)
	}
}

func TestParseEntityInvalidDefaultValue(t *testing.T) {
	input := []byte(`Order {
		processed bool yes
		}`)

	scanner := NewLineScanner(input)

	_, err := parseEntity(scanner)
	if !errors.Is(err, ErrIncorrectFormat) {
		t.Fatalf("parseEntity() error = %v, want ErrIncorrectFormat", err)
	}
}

func TestParseEntityUnexpectedTokenAfterFieldDefault(t *testing.T) {
	input := []byte(`Order {
		priority int64 5 extra
		}`)

	scanner := NewLineScanner(input)

	_, err := parseEntity(scanner)
	if !errors.Is(err, ErrIncorrectFormat) {
		t.Fatalf("parseEntity() error = %v, want ErrIncorrectFormat", err)
	}
}

func TestParseEntityMissingClosingBrace(t *testing.T) {
	input := []byte(`Order {
		priority int64
		`)

	scanner := NewLineScanner(input)

	_, err := parseEntity(scanner)
	if !errors.Is(err, ErrUnexpectedEOF) {
		t.Fatalf("parseEntity() error = %v, want ErrUnexpectedEOF", err)
	}
}

func TestParseEntityUnexpectedTokenAfterClosingBrace(t *testing.T) {
	input := []byte(`Order {
		priority int64
		} extra`)

	scanner := NewLineScanner(input)

	_, err := parseEntity(scanner)
	if !errors.Is(err, ErrIncorrectFormat) {
		t.Fatalf("parseEntity() error = %v, want ErrIncorrectFormat", err)
	}
}

func TestParseEntitySection(t *testing.T) {
	input := []byte(`Order {
		arrivalTime float64
		}
		Customer {
		priority uint64 1
		}
		}`)

	scanner := NewLineScanner(input)
	model := NewModel()
	model.Entities = make([]*EntityDesc, 0, 2)

	err := parseEntitySection(scanner, model, 2)
	if err != nil {
		t.Fatalf("parseEntitySection() error = %v", err)
	}

	if len(model.Entities) != 2 {
		t.Fatalf("len(model.Entities) = %d, want 2", len(model.Entities))
	}

	if model.Entities[0].Name != "Order" {
		t.Fatalf("model.Entities[0].Name = %q, want %q", model.Entities[0].Name, "Order")
	}

	if model.Entities[1].Name != "Customer" {
		t.Fatalf("model.Entities[1].Name = %q, want %q", model.Entities[1].Name, "Customer")
	}

	if model.EntityByName["Order"] != model.Entities[0] {
		t.Fatal(`model.EntityByName["Order"] does not point to first entity`)
	}

	if model.EntityByName["Customer"] != model.Entities[1] {
		t.Fatal(`model.EntityByName["Customer"] does not point to second entity`)
	}
}

func TestParseEntitySectionDuplicateGlobalName(t *testing.T) {
	input := []byte(`Order {
		arrivalTime float64
		}
		}`)

	scanner := NewLineScanner(input)
	model := NewModel()
	model.VarByName["Order"] = &VarDesc{Name: "Order"}
	model.Entities = make([]*EntityDesc, 0, 1)

	err := parseEntitySection(scanner, model, 1)
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("parseEntitySection() error = %v, want ErrDuplicateName", err)
	}
}
