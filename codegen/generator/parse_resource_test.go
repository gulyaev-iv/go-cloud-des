package generator

import (
	"errors"
	"testing"
)

func TestParseResource(t *testing.T) {
	input := []byte(`Server {
		capacity 4
		seize SeizeServer
		seize SeizeBackup
		}`)

	scanner := NewLineScanner(input)

	resource, err := parseResource(scanner)
	if err != nil {
		t.Fatalf("parseResource() error = %v", err)
	}

	if resource.Name != "Server" {
		t.Fatalf("resource.Name = %q, want %q", resource.Name, "Server")
	}

	if resource.Capacity != Expr("4") {
		t.Fatalf("resource.Capacity = %q, want %q", resource.Capacity, Expr("4"))
	}

	if len(resource.SeizeNames) != 2 {
		t.Fatalf("len(resource.SeizeNames) = %d, want 2", len(resource.SeizeNames))
	}

	if resource.SeizeNames[0] != "SeizeServer" {
		t.Fatalf("resource.SeizeNames[0] = %q, want %q", resource.SeizeNames[0], "SeizeServer")
	}

	if resource.SeizeNames[1] != "SeizeBackup" {
		t.Fatalf("resource.SeizeNames[1] = %q, want %q", resource.SeizeNames[1], "SeizeBackup")
	}
}

func TestParseResourceWithError(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{name: "UnexpectedEOF", input: nil, wantErr: ErrUnexpectedEOF},
		{name: "InvalidName", input: []byte("1Server {\ncapacity 4\nseize SeizeServer\n}"), wantErr: ErrInvalidIdentifier},
		{name: "ReservedKeywordName", input: []byte("ENTITY {\ncapacity 4\nseize SeizeServer\n}"), wantErr: ErrInvalidIdentifier},
		{name: "MissingOpeningBrace", input: []byte("Server \ncapacity 4\nseize SeizeServer\n}"), wantErr: ErrIncorrectFormat},
		{name: "UnexpectedTokenAfterOpeningBrace", input: []byte("Server { extra\ncapacity 4\nseize SeizeServer\n}"), wantErr: ErrIncorrectFormat},
		{name: "MissingCapacity", input: []byte("Server {\nseize SeizeServer\n}"), wantErr: ErrIncorrectFormat},
		{name: "MissingSeize", input: []byte("Server {\ncapacity 4\n}"), wantErr: ErrIncorrectFormat},
		{name: "DuplicateCapacity", input: []byte("Server {\ncapacity 4\ncapacity 5\nseize SeizeServer\n}"), wantErr: ErrIncorrectFormat},
		{name: "MissingCapacityValue", input: []byte("Server {\ncapacity \nseize SeizeServer\n}"), wantErr: ErrIncorrectFormat},
		{name: "UnexpectedTokenAfterCapacityValue", input: []byte("Server {\ncapacity 4 extra\nseize SeizeServer\n}"), wantErr: ErrIncorrectFormat},
		{name: "MissingSeizeName", input: []byte("Server {\ncapacity 4\n seize\n}"), wantErr: ErrIncorrectFormat},
		{name: "UnexpectedTokenAfterSeizeName", input: []byte("Server {\ncapacity 4\n seize SeizeServer extra\n}"), wantErr: ErrIncorrectFormat},
		{name: "InvalidSeizeName", input: []byte("Server {\ncapacity 4\n seize 1SeizeServer\n}"), wantErr: ErrInvalidIdentifier},
		{name: "UnknownParam", input: []byte("Server {\ncapacity 4\nseize SeizeServer\npriority 10\n}"), wantErr: ErrUnknownBlockParam},
		{name: "UnexpectedTokenAfterClosingBrace", input: []byte("Server {\ncapacity 4\nseize SeizeServer\n} extra"), wantErr: ErrIncorrectFormat},
		{name: "MissingClosingBrace", input: []byte("Server {\ncapacity 4\nseize SeizeServer"), wantErr: ErrUnexpectedEOF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewLineScanner(tt.input)

			_, err := parseResource(scanner)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("parseResource() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseResourceSection(t *testing.T) {
	input := []byte(`Server {
		capacity 4
		seize SeizeServer
		}
		Backup {
		capacity backupCapacity
		seize SeizeServer2
		}
		}`)

	scanner := NewLineScanner(input)
	model := NewModel()
	model.Resources = make([]*ResourceDesc, 0, 2)

	err := parseResourceSection(scanner, model, 2)
	if err != nil {
		t.Fatalf("parseResourceSection() error = %v", err)
	}

	if len(model.Resources) != 2 {
		t.Fatalf("len(model.Resources) = %d, want 2", len(model.Resources))
	}

	testResource(t, model, 0, "Server", Expr("4"), []string{"SeizeServer"})
	testResource(t, model, 1, "Backup", Expr("backupCapacity"), []string{"SeizeServer2"})
}

func testResource(t *testing.T, model *Model, index int, name string, capacity Expr, seizeNames []string) {
	t.Helper()

	if model.Resources[index].Name != name {
		t.Fatalf("model.Resources[%d].Name = %q, want %q", index, model.Resources[index].Name, name)
	}

	if model.Resources[index].Capacity != capacity {
		t.Fatalf("%s.Capacity = %q, want %q", name, model.Resources[index].Capacity, capacity)
	}

	if len(model.Resources[index].SeizeNames) != len(seizeNames) {
		t.Fatalf("len(%s.SeizeNames) = %d, want %d", name, len(model.Resources[index].SeizeNames), len(seizeNames))
	}

	for i := range seizeNames {
		if model.Resources[index].SeizeNames[i] != seizeNames[i] {
			t.Fatalf("%s.SeizeNames[%d] = %q, want %q", name, i, model.Resources[index].SeizeNames[i], seizeNames[i])
		}
	}

	if model.ResourceByName[name] != model.Resources[index] {
		t.Fatalf("model.ResourceByName[%q] does not point to model.Resources[%d]", name, index)
	}
}

func TestParseResourceSectionDuplicateGlobalName(t *testing.T) {
	input := []byte(`Server {
		capacity 4
		seize SeizeServer
		}
		}`)

	scanner := NewLineScanner(input)
	model := NewModel()
	model.VarByName["Server"] = &VarDesc{Name: "Server"}
	model.Resources = make([]*ResourceDesc, 0, 1)

	err := parseResourceSection(scanner, model, 1)
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("parseResourceSection() error = %v, want ErrDuplicateName", err)
	}
}

func TestParseResourceSectionMissingClosingBrace(t *testing.T) {
	input := []byte(`Server {
		capacity 4
		seize SeizeServer
		}`)

	scanner := NewLineScanner(input)
	model := NewModel()
	model.Resources = make([]*ResourceDesc, 0, 1)

	err := parseResourceSection(scanner, model, 1)
	if !errors.Is(err, ErrUnexpectedEOF) {
		t.Fatalf("parseResourceSection() error = %v, want ErrUnexpectedEOF", err)
	}
}

func TestParseResourceSectionTooManyObjects(t *testing.T) {
	input := []byte(`Server {
		capacity 4
		seize SiezeServer
		}
		Backup {
		capacity 2
		}
		}`)

	scanner := NewLineScanner(input)
	model := NewModel()
	model.Resources = make([]*ResourceDesc, 0, 1)

	err := parseResourceSection(scanner, model, 1)
	if !errors.Is(err, ErrIncorrectFormat) {
		t.Fatalf("parseResourceSection() error = %v, want ErrIncorrectFormat", err)
	}
}
