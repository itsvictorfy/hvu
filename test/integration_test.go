package test

import (
	"encoding/json"
	"os"
	"path/filepath"
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
