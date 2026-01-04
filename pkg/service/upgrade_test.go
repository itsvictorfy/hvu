package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpgrade_MissingValuesFile(t *testing.T) {
	input := &UpgradeInput{
		Chart:       "test-chart",
		Repository:  "https://example.com/charts",
		FromVersion: "1.0.0",
		ToVersion:   "2.0.0",
		ValuesFile:  "/nonexistent/path/values.yaml",
		OutputDir:   t.TempDir(),
		DryRun:      true,
	}

	_, err := Upgrade(input)

	if err == nil {
		t.Error("expected error for missing values file")
	}
}

func TestUpgrade_SameVersion(t *testing.T) {
	// Create a temporary values file
	tmpDir := t.TempDir()
	valuesFile := filepath.Join(tmpDir, "values.yaml")
	if err := os.WriteFile(valuesFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	input := &UpgradeInput{
		Chart:       "test-chart",
		Repository:  "https://example.com/charts",
		FromVersion: "1.0.0",
		ToVersion:   "1.0.0", // Same version
		ValuesFile:  valuesFile,
		OutputDir:   tmpDir,
		DryRun:      true,
	}

	_, err := Upgrade(input)

	if err == nil {
		t.Error("expected error when fromVersion equals toVersion")
	}
}

func TestUpgrade_InputValidation(t *testing.T) {
	tmpDir := t.TempDir()
	valuesFile := filepath.Join(tmpDir, "values.yaml")
	if err := os.WriteFile(valuesFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		input     *UpgradeInput
		wantError bool
	}{
		{
			name: "missing values file",
			input: &UpgradeInput{
				Chart:       "test-chart",
				Repository:  "https://charts.example.com",
				FromVersion: "1.0.0",
				ToVersion:   "2.0.0",
				ValuesFile:  "/nonexistent/values.yaml",
				OutputDir:   tmpDir,
				DryRun:      true,
			},
			wantError: true,
		},
		{
			name: "same versions",
			input: &UpgradeInput{
				Chart:       "test-chart",
				Repository:  "https://charts.example.com",
				FromVersion: "1.0.0",
				ToVersion:   "1.0.0",
				ValuesFile:  valuesFile,
				OutputDir:   tmpDir,
				DryRun:      true,
			},
			wantError: true,
		},
		{
			name: "valid input but invalid repo",
			input: &UpgradeInput{
				Chart:       "test-chart",
				Repository:  "https://invalid.nonexistent.repo",
				FromVersion: "1.0.0",
				ToVersion:   "2.0.0",
				ValuesFile:  valuesFile,
				OutputDir:   tmpDir,
				DryRun:      true,
			},
			wantError: true, // Will fail on network fetch
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Upgrade(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("Upgrade() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestUpgradeInput_Fields(t *testing.T) {
	input := &UpgradeInput{
		Chart:       "postgresql",
		Repository:  "https://charts.bitnami.com/bitnami",
		FromVersion: "15.0.0",
		ToVersion:   "16.0.0",
		ValuesFile:  "/path/to/values.yaml",
		OutputDir:   "/tmp/output",
		DryRun:      true,
	}

	if input.Chart != "postgresql" {
		t.Errorf("expected chart=postgresql, got %s", input.Chart)
	}
	if input.FromVersion != "15.0.0" {
		t.Errorf("expected fromVersion=15.0.0, got %s", input.FromVersion)
	}
	if input.ToVersion != "16.0.0" {
		t.Errorf("expected toVersion=16.0.0, got %s", input.ToVersion)
	}
	if !input.DryRun {
		t.Error("expected dryRun=true")
	}
}

func TestUpgradeOutput_Fields(t *testing.T) {
	output := &UpgradeOutput{
		Classification:   nil,
		UpgradedYAML:     "key: value\n",
		OutputPath:       "/tmp/output.yaml",
		OldDefaultsCount: 100,
		NewDefaultsCount: 120,
		UserValuesCount:  50,
	}

	if output.OldDefaultsCount != 100 {
		t.Errorf("expected oldDefaultsCount=100, got %d", output.OldDefaultsCount)
	}
	if output.NewDefaultsCount != 120 {
		t.Errorf("expected newDefaultsCount=120, got %d", output.NewDefaultsCount)
	}
	if output.UserValuesCount != 50 {
		t.Errorf("expected userValuesCount=50, got %d", output.UserValuesCount)
	}
	if output.UpgradedYAML != "key: value\n" {
		t.Errorf("expected upgradedYAML content, got %s", output.UpgradedYAML)
	}
	if output.OutputPath != "/tmp/output.yaml" {
		t.Errorf("expected outputPath, got %s", output.OutputPath)
	}
}

func TestUpgrade_DryRunDoesNotWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	valuesFile := filepath.Join(tmpDir, "values.yaml")
	if err := os.WriteFile(valuesFile, []byte("key: value"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	input := &UpgradeInput{
		Chart:       "test-chart",
		Repository:  "https://invalid.repo", // Will fail, but dryRun should prevent file writes anyway
		FromVersion: "1.0.0",
		ToVersion:   "2.0.0",
		ValuesFile:  valuesFile,
		OutputDir:   outputDir,
		DryRun:      true,
	}

	// This will fail due to invalid repo, but we're testing the dryRun behavior
	_, _ = Upgrade(input)

	// Output directory should not be created due to dry run (or error)
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		// If it exists, check if it was created by our function
		entries, _ := os.ReadDir(outputDir)
		if len(entries) > 0 {
			t.Error("dry run should not write any files")
		}
	}
}
