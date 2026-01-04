package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassify_MissingValuesFile(t *testing.T) {
	input := &ClassifyInput{
		Chart:      "test-chart",
		Repository: "https://example.com/charts",
		Version:    "1.0.0",
		ValuesFile: "/nonexistent/path/values.yaml",
	}

	_, err := Classify(input)

	if err == nil {
		t.Error("expected error for missing values file")
	}
}

func TestClassify_InputValidation(t *testing.T) {
	// Create a temporary values file
	tmpDir := t.TempDir()
	valuesFile := filepath.Join(tmpDir, "values.yaml")
	if err := os.WriteFile(valuesFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		input     *ClassifyInput
		wantError bool
	}{
		{
			name: "valid input structure",
			input: &ClassifyInput{
				Chart:      "test-chart",
				Repository: "https://charts.example.com",
				Version:    "1.0.0",
				ValuesFile: valuesFile,
			},
			// Will fail on network, but validates input structure
			wantError: true,
		},
		{
			name: "missing values file",
			input: &ClassifyInput{
				Chart:      "test-chart",
				Repository: "https://charts.example.com",
				Version:    "1.0.0",
				ValuesFile: "/nonexistent/values.yaml",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Classify(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("Classify() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestClassifyInput_Fields(t *testing.T) {
	input := &ClassifyInput{
		Chart:      "grafana",
		Repository: "https://grafana.github.io/helm-charts",
		Version:    "8.0.0",
		ValuesFile: "/path/to/values.yaml",
	}

	if input.Chart != "grafana" {
		t.Errorf("expected chart=grafana, got %s", input.Chart)
	}
	if input.Repository != "https://grafana.github.io/helm-charts" {
		t.Errorf("expected repository URL, got %s", input.Repository)
	}
	if input.Version != "8.0.0" {
		t.Errorf("expected version=8.0.0, got %s", input.Version)
	}
	if input.ValuesFile != "/path/to/values.yaml" {
		t.Errorf("expected valuesFile path, got %s", input.ValuesFile)
	}
}

func TestClassifyOutput_Fields(t *testing.T) {
	output := &ClassifyOutput{
		Result:        nil,
		DefaultsCount: 100,
		UserCount:     50,
	}

	if output.DefaultsCount != 100 {
		t.Errorf("expected defaultsCount=100, got %d", output.DefaultsCount)
	}
	if output.UserCount != 50 {
		t.Errorf("expected userCount=50, got %d", output.UserCount)
	}
}
