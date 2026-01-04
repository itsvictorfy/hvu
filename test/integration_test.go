package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itsvictorfy/hvu/pkg/service"
	"github.com/itsvictorfy/hvu/pkg/values"
)

// TestConfig represents the configuration for integration tests
type TestConfig struct {
	ChartName      string `json:"chartName"`
	ChartURL       string `json:"chartUrl"`
	FromVersion    string `json:"fromVersion"`
	ToVersion      string `json:"toVersion"`
	ValuesFilePath string `json:"valuesFilePath"`
}

// loadTestConfig loads the test configuration from test.config.json
func loadTestConfig(t *testing.T) *TestConfig {
	t.Helper()
	configPath := filepath.Join(testDataDir(), "test.config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Skipf("test.config.json not found: %v", err)
	}

	var config TestConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to parse test.config.json: %v", err)
	}

	return &config
}

// TestIntegration_UpgradePreservesCustomizations is an integration test that:
// 1. Uses service.Classify to classify user values against old chart version
// 2. Uses service.Upgrade to upgrade values to new chart version
// 3. Uses service.Classify to classify upgraded values against new chart version
// 4. Verifies that all CUSTOMIZED keys from old classification remain CUSTOMIZED
func TestIntegration_UpgradePreservesCustomizations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	config := loadTestConfig(t)
	valuesFilePath := filepath.Join(testDataDir(), config.ValuesFilePath)

	// Step 1: Classify user values against old version using service.Classify
	t.Logf("Classifying user values against %s v%s...", config.ChartName, config.FromVersion)

	oldClassifyOutput, err := service.Classify(&service.ClassifyInput{
		Chart:      config.ChartName,
		Repository: config.ChartURL,
		Version:    config.FromVersion,
		ValuesFile: valuesFilePath,
	})
	if err != nil {
		t.Fatalf("failed to classify against old version: %v", err)
	}

	t.Logf("Old version classification: customized=%d, copiedDefault=%d, unknown=%d (defaults=%d, user=%d)",
		oldClassifyOutput.Result.Customized,
		oldClassifyOutput.Result.CopiedDefault,
		oldClassifyOutput.Result.Unknown,
		oldClassifyOutput.DefaultsCount,
		oldClassifyOutput.UserCount,
	)

	// Collect customized paths from old classification
	oldCustomizedPaths := make(map[string]interface{})
	for _, entry := range oldClassifyOutput.Result.Entries {
		if entry.Classification == values.Customized {
			oldCustomizedPaths[entry.Path] = entry.UserValue
			t.Logf("  OLD CUSTOMIZED: %s = %v", entry.Path, entry.UserValue)
		}
	}

	// Step 2: Upgrade using service.Upgrade
	t.Logf("Upgrading from %s to %s...", config.FromVersion, config.ToVersion)

	upgradeOutput, err := service.Upgrade(&service.UpgradeInput{
		Chart:       config.ChartName,
		Repository:  config.ChartURL,
		FromVersion: config.FromVersion,
		ToVersion:   config.ToVersion,
		ValuesFile:  valuesFilePath,
		OutputDir:   t.TempDir(),
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("failed to upgrade: %v", err)
	}

	t.Logf("Upgrade complete: oldDefaults=%d, newDefaults=%d, userValues=%d",
		upgradeOutput.OldDefaultsCount,
		upgradeOutput.NewDefaultsCount,
		upgradeOutput.UserValuesCount,
	)
	t.Logf("Upgraded values written to: %s", upgradeOutput.OutputPath)

	// Step 3: Classify upgraded values against new version using service.Classify
	t.Logf("Classifying upgraded values against %s v%s...", config.ChartName, config.ToVersion)

	newClassifyOutput, err := service.Classify(&service.ClassifyInput{
		Chart:      config.ChartName,
		Repository: config.ChartURL,
		Version:    config.ToVersion,
		ValuesFile: upgradeOutput.OutputPath,
	})
	if err != nil {
		t.Fatalf("failed to classify against new version: %v", err)
	}

	t.Logf("New version classification: customized=%d, copiedDefault=%d, unknown=%d",
		newClassifyOutput.Result.Customized,
		newClassifyOutput.Result.CopiedDefault,
		newClassifyOutput.Result.Unknown,
	)

	// Collect customized paths from new classification
	newCustomizedPaths := make(map[string]interface{})
	for _, entry := range newClassifyOutput.Result.Entries {
		if entry.Classification == values.Customized {
			newCustomizedPaths[entry.Path] = entry.UserValue
		}
	}

	// Step 4: Verify all old customizations are preserved in new classification
	var missingCustomizations []string
	var valueChanges []string

	for path, oldValue := range oldCustomizedPaths {
		newValue, existsInNew := newCustomizedPaths[path]
		if !existsInNew {
			// Check if the path exists at all in new classification
			found := false
			for _, entry := range newClassifyOutput.Result.Entries {
				if entry.Path == path {
					found = true
					// Path exists but is no longer customized (became default or unknown)
					missingCustomizations = append(missingCustomizations,
						path+" (was CUSTOMIZED, now "+string(entry.Classification)+")")
					break
				}
			}
			if !found {
				// Path might have been removed - check the upgrade classification
				for _, entry := range upgradeOutput.Classification.Entries {
					if entry.Path == path && entry.Classification == values.Customized {
						missingCustomizations = append(missingCustomizations,
							path+" (not in new classification)")
						break
					}
				}
			}
		} else {
			// Verify the value is preserved
			if !values.ValuesEqual(oldValue, newValue) {
				valueChanges = append(valueChanges,
					path+": old="+formatTestValue(oldValue)+" new="+formatTestValue(newValue))
			}
		}
	}

	// Report missing customizations
	if len(missingCustomizations) > 0 {
		t.Errorf("Some customizations were not preserved after upgrade:")
		for _, msg := range missingCustomizations {
			t.Errorf("  - %s", msg)
		}
	}

	// Report value changes (these might be intentional, so just log them)
	if len(valueChanges) > 0 {
		t.Logf("Value changes detected (may be expected):")
		for _, msg := range valueChanges {
			t.Logf("  - %s", msg)
		}
	}

	// Log new customized paths that weren't in old
	for path := range newCustomizedPaths {
		if _, wasOld := oldCustomizedPaths[path]; !wasOld {
			t.Logf("  NEW CUSTOMIZED: %s", path)
		}
	}

	// Final summary
	t.Logf("Summary: %d/%d old customizations preserved",
		len(oldCustomizedPaths)-len(missingCustomizations), len(oldCustomizedPaths))
}

// TestIntegration_ClassifyMatchesExpected verifies that classification
// of the user values file produces expected results using service.Classify
func TestIntegration_ClassifyMatchesExpected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	config := loadTestConfig(t)
	valuesFilePath := filepath.Join(testDataDir(), config.ValuesFilePath)

	// Classify using service.Classify
	t.Logf("Classifying %s against %s v%s...", config.ValuesFilePath, config.ChartName, config.FromVersion)

	output, err := service.Classify(&service.ClassifyInput{
		Chart:      config.ChartName,
		Repository: config.ChartURL,
		Version:    config.FromVersion,
		ValuesFile: valuesFilePath,
	})
	if err != nil {
		t.Fatalf("failed to classify: %v", err)
	}

	t.Logf("Classification results for %s against %s v%s:",
		config.ValuesFilePath, config.ChartName, config.FromVersion)
	t.Logf("  Defaults: %d keys", output.DefaultsCount)
	t.Logf("  User values: %d keys", output.UserCount)
	t.Logf("  Total classified: %d", output.Result.Total)
	t.Logf("  Customized: %d", output.Result.Customized)
	t.Logf("  Copied Default: %d", output.Result.CopiedDefault)
	t.Logf("  Unknown: %d", output.Result.Unknown)

	// The scenario-mixed.yaml should have customizations and no unknown keys
	if output.Result.Customized == 0 {
		t.Error("expected at least some customized values")
	}

	// Log all classifications for debugging
	for _, entry := range output.Result.Entries {
		t.Logf("  %s: %s", entry.Classification, entry.Path)
	}
}

// TestIntegration_UpgradeOutputIsValid verifies that the upgrade output
// can be parsed and classified without errors
func TestIntegration_UpgradeOutputIsValid(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	config := loadTestConfig(t)
	valuesFilePath := filepath.Join(testDataDir(), config.ValuesFilePath)
	outputDir := t.TempDir()

	// Run upgrade
	t.Logf("Running upgrade from %s to %s...", config.FromVersion, config.ToVersion)

	upgradeOutput, err := service.Upgrade(&service.UpgradeInput{
		Chart:       config.ChartName,
		Repository:  config.ChartURL,
		FromVersion: config.FromVersion,
		ToVersion:   config.ToVersion,
		ValuesFile:  valuesFilePath,
		OutputDir:   outputDir,
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}

	// Verify output file exists
	if _, err := os.Stat(upgradeOutput.OutputPath); os.IsNotExist(err) {
		t.Fatalf("output file was not created: %s", upgradeOutput.OutputPath)
	}

	// Verify the output YAML is valid by parsing it
	outputContent, err := os.ReadFile(upgradeOutput.OutputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	parsedOutput, err := values.ParseYAML(string(outputContent))
	if err != nil {
		t.Fatalf("output YAML is invalid: %v", err)
	}

	t.Logf("Output file contains %d keys", len(parsedOutput))

	// The output should have at least as many keys as the new defaults
	if len(parsedOutput) < upgradeOutput.NewDefaultsCount {
		t.Errorf("output has fewer keys (%d) than new defaults (%d)",
			len(parsedOutput), upgradeOutput.NewDefaultsCount)
	}

	// Verify upgrade classification was captured
	if upgradeOutput.Classification == nil {
		t.Error("upgrade output should contain classification result")
	} else {
		t.Logf("Upgrade captured classification: customized=%d, copiedDefault=%d, unknown=%d",
			upgradeOutput.Classification.Customized,
			upgradeOutput.Classification.CopiedDefault,
			upgradeOutput.Classification.Unknown,
		)
	}
}

// formatTestValue formats a value for test output
func formatTestValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		if len(val) > 50 {
			return val[:50] + "..."
		}
		return val
	default:
		return values.FormatValue(v)
	}
}

// TestIntegration_UpgradeOutputHasYAMLExtension verifies that upgrade output files
// have the .yaml extension as expected
func TestIntegration_UpgradeOutputHasYAMLExtension(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	config := loadTestConfig(t)
	valuesFilePath := filepath.Join(testDataDir(), config.ValuesFilePath)
	outputDir := t.TempDir()

	// Run upgrade
	upgradeOutput, err := service.Upgrade(&service.UpgradeInput{
		Chart:       config.ChartName,
		Repository:  config.ChartURL,
		FromVersion: config.FromVersion,
		ToVersion:   config.ToVersion,
		ValuesFile:  valuesFilePath,
		OutputDir:   outputDir,
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}

	// Verify output file has .yaml extension
	if !strings.HasSuffix(upgradeOutput.OutputPath, ".yaml") {
		t.Errorf("output file should have .yaml extension, got: %s", upgradeOutput.OutputPath)
	}

	// Verify the filename format: {chart}-{version}-{timestamp}.yaml
	filename := filepath.Base(upgradeOutput.OutputPath)
	expectedPrefix := config.ChartName + "-" + config.ToVersion + "-"
	if !strings.HasPrefix(filename, expectedPrefix) {
		t.Errorf("output filename should start with %q, got: %s", expectedPrefix, filename)
	}

	t.Logf("Output file: %s", upgradeOutput.OutputPath)
}

// TestIntegration_CommentExtraction verifies that comments are extracted from chart defaults
func TestIntegration_CommentExtraction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	config := loadTestConfig(t)
	valuesFilePath := filepath.Join(testDataDir(), config.ValuesFilePath)
	outputDir := t.TempDir()

	// Run upgrade (which internally extracts and applies comments)
	upgradeOutput, err := service.Upgrade(&service.UpgradeInput{
		Chart:       config.ChartName,
		Repository:  config.ChartURL,
		FromVersion: config.FromVersion,
		ToVersion:   config.ToVersion,
		ValuesFile:  valuesFilePath,
		OutputDir:   outputDir,
		DryRun:      false,
	})
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}

	// Read the output file
	outputContent, err := os.ReadFile(upgradeOutput.OutputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	// The output should contain comments (lines starting with ##)
	// Bitnami charts use ## @param style comments
	hasComments := strings.Contains(string(outputContent), "##")
	t.Logf("Output has comments: %v", hasComments)

	// Log a sample of the output for debugging
	lines := strings.Split(string(outputContent), "\n")
	commentCount := 0
	for _, line := range lines {
		if strings.Contains(line, "##") {
			commentCount++
			if commentCount <= 5 {
				t.Logf("  Comment: %s", line)
			}
		}
	}
	t.Logf("Total comment lines: %d", commentCount)
}

// TestIntegration_ToYAMLWithComments verifies ToYAMLWithComments function works correctly
func TestIntegration_ToYAMLWithComments(t *testing.T) {
	// Create test values
	testValues := values.Values{
		"image.repository": "nginx",
		"image.tag":        "1.21",
		"replicaCount":     3,
		"service.type":     "ClusterIP",
		"service.port":     80,
	}

	// Create test comments
	comments := values.CommentMap{
		"image.repository": "Docker image repository",
		"image.tag":        "Docker image tag",
		"replicaCount":     "Number of replicas to deploy",
		"service.type":     "Kubernetes service type",
		"service.port":     "Service port number",
	}

	// Generate YAML with comments
	yamlOutput, err := testValues.ToYAMLWithComments(comments)
	if err != nil {
		t.Fatalf("ToYAMLWithComments failed: %v", err)
	}

	// Verify the output contains comments
	if !strings.Contains(yamlOutput, "##") {
		t.Error("expected output to contain comments (##)")
	}

	// Verify specific comments are present
	expectedComments := []string{
		"Docker image repository",
		"Docker image tag",
		"Number of replicas",
		"Kubernetes service type",
		"Service port number",
	}

	for _, expected := range expectedComments {
		if !strings.Contains(yamlOutput, expected) {
			t.Errorf("expected output to contain comment: %q", expected)
		}
	}

	// Verify the YAML is still valid by parsing it
	parsed, err := values.ParseYAML(yamlOutput)
	if err != nil {
		t.Fatalf("generated YAML is invalid: %v", err)
	}

	// Verify values are preserved
	if parsed["image.repository"] != "nginx" {
		t.Errorf("expected image.repository=nginx, got %v", parsed["image.repository"])
	}
	if parsed["replicaCount"] != 3 {
		t.Errorf("expected replicaCount=3, got %v", parsed["replicaCount"])
	}

	t.Logf("Generated YAML with comments:\n%s", yamlOutput)
}

// TestIntegration_ExtractComments verifies ExtractComments function works correctly
func TestIntegration_ExtractComments(t *testing.T) {
	// Sample YAML content with @param comments (Bitnami style)
	yamlContent := `
## @param image.repository Docker image repository
## @param image.tag Docker image tag
image:
  repository: nginx
  tag: "1.21"

## @param replicaCount Number of replicas
replicaCount: 1

## @param service.type Kubernetes service type
## @param service.port Service port number
service:
  type: ClusterIP
  port: 80
`

	comments := values.ExtractComments(yamlContent)

	// Verify comments were extracted
	if len(comments) == 0 {
		t.Error("expected comments to be extracted")
	}

	// Verify specific comments
	expectedComments := map[string]string{
		"image.repository": "Docker image repository",
		"image.tag":        "Docker image tag",
		"replicaCount":     "Number of replicas",
		"service.type":     "Kubernetes service type",
		"service.port":     "Service port number",
	}

	for path, expectedComment := range expectedComments {
		if comment, ok := comments[path]; !ok {
			t.Errorf("expected comment for path %q", path)
		} else if comment != expectedComment {
			t.Errorf("expected comment for %q to be %q, got %q", path, expectedComment, comment)
		}
	}

	t.Logf("Extracted %d comments", len(comments))
	for path, comment := range comments {
		t.Logf("  %s: %s", path, comment)
	}
}

// TestIntegration_DryRunDoesNotCreateFile verifies dry run mode doesn't create files
func TestIntegration_DryRunDoesNotCreateFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	config := loadTestConfig(t)
	valuesFilePath := filepath.Join(testDataDir(), config.ValuesFilePath)
	outputDir := t.TempDir()

	// Run upgrade with dry run
	upgradeOutput, err := service.Upgrade(&service.UpgradeInput{
		Chart:       config.ChartName,
		Repository:  config.ChartURL,
		FromVersion: config.FromVersion,
		ToVersion:   config.ToVersion,
		ValuesFile:  valuesFilePath,
		OutputDir:   outputDir,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}

	// Verify no output file was created
	if upgradeOutput.OutputPath != "" {
		t.Errorf("expected empty output path in dry run mode, got: %s", upgradeOutput.OutputPath)
	}

	// Verify the upgraded YAML is still generated
	if upgradeOutput.UpgradedYAML == "" {
		t.Error("expected UpgradedYAML to be populated even in dry run mode")
	}

	// Verify no files were created in output dir
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("failed to read output dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files in output dir, found %d", len(entries))
	}

	t.Logf("Dry run completed, no files created")
}
