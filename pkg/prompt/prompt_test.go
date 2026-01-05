package prompt

import (
	"bytes"
	"strings"
	"testing"

	"github.com/itsvictorfy/hvu/pkg/values"
)

func TestConfirmImageUpgrade_Yes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"lowercase y", "y\n", true},
		{"uppercase Y", "Y\n", true},
		{"lowercase yes", "yes\n", true},
		{"uppercase YES", "YES\n", true},
		{"mixed case Yes", "Yes\n", true},
		{"with whitespace", "  y  \n", true},
	}

	changes := []values.ImageChange{
		{
			Path:       "image::tag",
			UserTag:    "1.5.0",
			OldDefault: "1.0.0",
			NewDefault: "2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			writer := &bytes.Buffer{}
			prompter := NewPrompterWithIO(reader, writer)

			result, err := prompter.ConfirmImageUpgrade(changes)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConfirmImageUpgrade_No(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"lowercase n", "n\n", false},
		{"uppercase N", "N\n", false},
		{"lowercase no", "no\n", false},
		{"uppercase NO", "NO\n", false},
		{"empty input", "\n", false},
		{"random text", "maybe\n", false},
		{"with whitespace", "  n  \n", false},
	}

	changes := []values.ImageChange{
		{
			Path:       "image::tag",
			UserTag:    "1.5.0",
			OldDefault: "1.0.0",
			NewDefault: "2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			writer := &bytes.Buffer{}
			prompter := NewPrompterWithIO(reader, writer)

			result, err := prompter.ConfirmImageUpgrade(changes)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConfirmImageUpgrade_EmptyChanges(t *testing.T) {
	reader := strings.NewReader("y\n")
	writer := &bytes.Buffer{}
	prompter := NewPrompterWithIO(reader, writer)

	result, err := prompter.ConfirmImageUpgrade([]values.ImageChange{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != false {
		t.Error("expected false for empty changes")
	}

	// Should not have written anything
	if writer.Len() != 0 {
		t.Errorf("expected no output for empty changes, got: %s", writer.String())
	}
}

func TestConfirmImageUpgrade_OutputFormat(t *testing.T) {
	changes := []values.ImageChange{
		{
			Path:       "image::tag",
			UserTag:    "1.5.0",
			OldDefault: "1.0.0",
			NewDefault: "2.0.0",
		},
		{
			Path:       "controller::image::tag",
			UserTag:    "v2.1.0",
			OldDefault: "v2.0.0",
			NewDefault: "v3.0.0",
		},
	}

	reader := strings.NewReader("n\n")
	writer := &bytes.Buffer{}
	prompter := NewPrompterWithIO(reader, writer)

	_, err := prompter.ConfirmImageUpgrade(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := writer.String()

	// Check for expected content
	expectedStrings := []string{
		"Custom image tags detected",
		"image.tag",            // display format uses dots
		"controller.image.tag", // display format uses dots
		"1.5.0",                // user tag
		"1.0.0",                // old default
		"2.0.0",                // new default
		"v2.1.0",               // user tag
		"v2.0.0",               // old default
		"v3.0.0",               // new default
		"Would you like to upgrade image tags to new defaults? [y/N]:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestConfirmImageUpgrade_SingleChange(t *testing.T) {
	changes := []values.ImageChange{
		{
			Path:       "image::tag",
			UserTag:    "custom",
			OldDefault: "old",
			NewDefault: "new",
		},
	}

	reader := strings.NewReader("y\n")
	writer := &bytes.Buffer{}
	prompter := NewPrompterWithIO(reader, writer)

	result, err := prompter.ConfirmImageUpgrade(changes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result {
		t.Error("expected true for 'y' input")
	}

	output := writer.String()
	if !strings.Contains(output, "image.tag") {
		t.Error("expected output to contain image.tag")
	}
}

func TestConfirmImageUpgrade_EOF(t *testing.T) {
	// Empty reader simulates EOF
	reader := strings.NewReader("")
	writer := &bytes.Buffer{}
	prompter := NewPrompterWithIO(reader, writer)

	changes := []values.ImageChange{
		{Path: "image::tag", UserTag: "1.0.0", OldDefault: "1.0.0", NewDefault: "2.0.0"},
	}

	result, err := prompter.ConfirmImageUpgrade(changes)
	if err != nil {
		t.Fatalf("unexpected error on EOF: %v", err)
	}

	// EOF should be treated as "no"
	if result != false {
		t.Error("expected false on EOF")
	}
}
