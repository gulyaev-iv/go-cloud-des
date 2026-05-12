package generator

import (
	"errors"
	"strings"
	"testing"
)

const validHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestParseHash(t *testing.T) {
	t.Run("valid hash", func(t *testing.T) {
		scanner := NewLineScanner([]byte(validHash))
		model := NewModel()

		err := parseHash(scanner, model)
		if err != nil {
			t.Fatalf("parseHash() error = %v", err)
		}

		if model.Hash != validHash {
			t.Fatalf("model.Hash = %q, want %q", model.Hash, validHash)
		}
	})

	t.Run("valid uppercase hash", func(t *testing.T) {
		hash := strings.ToUpper(validHash)

		scanner := NewLineScanner([]byte(hash))
		model := NewModel()

		err := parseHash(scanner, model)
		if err != nil {
			t.Fatalf("parseHash() error = %v", err)
		}

		if model.Hash != hash {
			t.Fatalf("model.Hash = %q, want %q", model.Hash, hash)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		scanner := NewLineScanner(nil)
		model := NewModel()

		err := parseHash(scanner, model)
		if !errors.Is(err, ErrUnexpectedEOF) {
			t.Fatalf("parseHash() error = %v, want ErrUnexpectedEOF", err)
		}
	})

	t.Run("short hash", func(t *testing.T) {
		hash := validHash[0 : len(validHash)/2]
		scanner := NewLineScanner([]byte(hash))
		model := NewModel()

		err := parseHash(scanner, model)
		if !errors.Is(err, ErrIncorrectHash) {
			t.Fatalf("parseHash() error = %v, want ErrIncorrectHash", err)
		}
	})

	t.Run("non hex hash", func(t *testing.T) {
		hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeg"

		scanner := NewLineScanner([]byte(hash))
		model := NewModel()

		err := parseHash(scanner, model)
		if !errors.Is(err, ErrIncorrectHash) {
			t.Fatalf("parseHash() error = %v, want ErrIncorrectHash", err)
		}
	})

	t.Run("extra token", func(t *testing.T) {
		scanner := NewLineScanner([]byte(validHash + " extra"))
		model := NewModel()

		err := parseHash(scanner, model)
		if !errors.Is(err, ErrIncorrectHash) {
			t.Fatalf("parseHash() error = %v, want ErrIncorrectHash", err)
		}
	})

	t.Run("skip empty lines before hash", func(t *testing.T) {
		scanner := NewLineScanner([]byte("\n   \n" + validHash))
		model := NewModel()

		err := parseHash(scanner, model)
		if err != nil {
			t.Fatalf("parseHash() error = %v", err)
		}

		if model.Hash != validHash {
			t.Fatalf("model.Hash = %q, want %q", model.Hash, validHash)
		}
	})
}

func TestParseSectionHeader(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		scanner := NewLineScanner([]byte("ENTITY 3 {"))

		count, err := parseSectionHeader(scanner, "ENTITY")
		if err != nil {
			t.Fatalf("parseSectionHeader() error = %v", err)
		}

		if count != 3 {
			t.Fatalf("count = %d, want 3", count)
		}
	})

	t.Run("valid zero count", func(t *testing.T) {
		scanner := NewLineScanner([]byte("VAR 0 {"))

		count, err := parseSectionHeader(scanner, "VAR")
		if err != nil {
			t.Fatalf("parseSectionHeader() error = %v", err)
		}

		if count != 0 {
			t.Fatalf("count = %d, want 0", count)
		}
	})

	t.Run("skip empty lines before header", func(t *testing.T) {
		scanner := NewLineScanner([]byte("\n  \nRESOURCE 2 {"))

		count, err := parseSectionHeader(scanner, "RESOURCE")
		if err != nil {
			t.Fatalf("parseSectionHeader() error = %v", err)
		}

		if count != 2 {
			t.Fatalf("count = %d, want 2", count)
		}
	})

	t.Run("unexpected eof", func(t *testing.T) {
		scanner := NewLineScanner(nil)

		_, err := parseSectionHeader(scanner, "ENTITY")
		if !errors.Is(err, ErrUnexpectedEOF) {
			t.Fatalf("parseSectionHeader() error = %v, want ErrUnexpectedEOF", err)
		}
	})

	t.Run("wrong section name", func(t *testing.T) {
		scanner := NewLineScanner([]byte("VAR 1 {"))

		_, err := parseSectionHeader(scanner, "ENTITY")
		if !errors.Is(err, ErrIncorrectFormat) {
			t.Fatalf("parseSectionHeader() error = %v, want ErrIncorrectFormat", err)
		}
	})

	t.Run("missing count", func(t *testing.T) {
		scanner := NewLineScanner([]byte("ENTITY"))

		_, err := parseSectionHeader(scanner, "ENTITY")
		if !errors.Is(err, ErrIncorrectFormat) {
			t.Fatalf("parseSectionHeader() error = %v, want ErrIncorrectFormat", err)
		}
	})

	t.Run("non numeric count", func(t *testing.T) {
		scanner := NewLineScanner([]byte("ENTITY abc {"))

		_, err := parseSectionHeader(scanner, "ENTITY")
		if !errors.Is(err, ErrIncorrectFormat) {
			t.Fatalf("parseSectionHeader() error = %v, want ErrIncorrectFormat", err)
		}
	})

	t.Run("negative count", func(t *testing.T) {
		scanner := NewLineScanner([]byte("ENTITY -1 {"))

		_, err := parseSectionHeader(scanner, "ENTITY")
		if !errors.Is(err, ErrIncorrectFormat) {
			t.Fatalf("parseSectionHeader() error = %v, want ErrIncorrectFormat", err)
		}
	})

	t.Run("missing opening brace", func(t *testing.T) {
		scanner := NewLineScanner([]byte("ENTITY 1"))

		_, err := parseSectionHeader(scanner, "ENTITY")
		if !errors.Is(err, ErrIncorrectFormat) {
			t.Fatalf("parseSectionHeader() error = %v, want ErrIncorrectFormat", err)
		}
	})

	t.Run("wrong opening brace", func(t *testing.T) {
		scanner := NewLineScanner([]byte("ENTITY 1 }"))

		_, err := parseSectionHeader(scanner, "ENTITY")
		if !errors.Is(err, ErrIncorrectFormat) {
			t.Fatalf("parseSectionHeader() error = %v, want ErrIncorrectFormat", err)
		}
	})

	t.Run("extra token after opening brace", func(t *testing.T) {
		scanner := NewLineScanner([]byte("ENTITY 1 { extra"))

		_, err := parseSectionHeader(scanner, "ENTITY")
		if !errors.Is(err, ErrIncorrectFormat) {
			t.Fatalf("parseSectionHeader() error = %v, want ErrIncorrectFormat", err)
		}
	})
}

func TestIsHex(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "digits", in: "0123456789", want: true},
		{name: "lowercase", in: "abcdef", want: true},
		{name: "uppercase", in: "ABCDEF", want: true},
		{name: "mixed", in: "0123abcABC", want: true},

		{name: "empty", in: "", want: false},
		{name: "invalid letter", in: "g", want: false},
		{name: "space", in: "abc def", want: false},
		{name: "dash", in: "abc-def", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isHex([]byte(tt.in))
			if got != tt.want {
				t.Fatalf("isHex(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
