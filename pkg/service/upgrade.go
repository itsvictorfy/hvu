package service

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/itsvictorfy/hvu/pkg/helm"
	"github.com/itsvictorfy/hvu/pkg/values"
)

// UpgradeInput contains input parameters for upgrade
type UpgradeInput struct {
	Chart       string
	Repository  string
	FromVersion string
	ToVersion   string
	ValuesFile  string
	OutputDir   string
	DryRun      bool
}

// UpgradeOutput contains the results of upgrade
type UpgradeOutput struct {
	Classification   *values.ClassificationResult
	UpgradedYAML     string
	OutputPath       string
	OldDefaultsCount int
	NewDefaultsCount int
	UserValuesCount  int
}

// Upgrade runs the upgrade logic
func Upgrade(input *UpgradeInput) (*UpgradeOutput, error) {
	slog.Debug("starting upgrade",
		"chart", input.Chart,
		"repository", input.Repository,
		"fromVersion", input.FromVersion,
		"toVersion", input.ToVersion,
		"valuesFile", input.ValuesFile,
		"outputDir", input.OutputDir,
		"dryRun", input.DryRun,
	)

	// Validate values file exists
	if _, err := os.Stat(input.ValuesFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("values file not found: %s", input.ValuesFile)
	}

	// Validate versions are different
	if input.FromVersion == input.ToVersion {
		return nil, fmt.Errorf("source and target versions are identical: %s", input.FromVersion)
	}

	// Step 1: Fetch old chart defaults
	slog.Debug("fetching old chart defaults", "version", input.FromVersion)

	oldDefaultsYAML, err := helm.GetValuesFileByVersion(input.Repository, input.Chart, input.FromVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch old chart defaults: %w", err)
	}

	oldDefaults, err := values.ParseYAML(oldDefaultsYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse old chart defaults: %w", err)
	}

	slog.Debug("parsed old defaults", "count", len(oldDefaults))

	// Step 2: Fetch new chart defaults
	slog.Debug("fetching new chart defaults", "version", input.ToVersion)

	newDefaultsYAML, err := helm.GetValuesFileByVersion(input.Repository, input.Chart, input.ToVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch new chart defaults: %w", err)
	}

	newDefaults, err := values.ParseYAML(newDefaultsYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new chart defaults: %w", err)
	}

	slog.Debug("parsed new defaults", "count", len(newDefaults))

	// Step 3: Parse user values
	slog.Debug("parsing user values", "file", input.ValuesFile)

	userValues, err := values.ParseFile(input.ValuesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user values: %w", err)
	}

	slog.Debug("parsed user values", "count", len(userValues))

	// Step 4: Classify user values against old defaults
	slog.Debug("classifying user values")

	classification := values.Classify(userValues, oldDefaults)

	slog.Debug("classification complete",
		"customized", classification.Customized,
		"copiedDefault", classification.CopiedDefault,
		"unknown", classification.Unknown,
	)

	// Step 5: Merge values
	slog.Debug("generating upgraded values")

	upgradedValues := values.Merge(userValues, oldDefaults, newDefaults)

	// Generate YAML output
	upgradedYAML, err := upgradedValues.ToYAML()
	if err != nil {
		return nil, fmt.Errorf("failed to generate YAML: %w", err)
	}

	output := &UpgradeOutput{
		Classification:   classification,
		UpgradedYAML:     upgradedYAML,
		OldDefaultsCount: len(oldDefaults),
		NewDefaultsCount: len(newDefaults),
		UserValuesCount:  len(userValues),
	}

	// Step 6: Write output (unless dry run)
	if !input.DryRun {
		slog.Debug("writing output file", "dir", input.OutputDir)

		// Create output directory if needed
		if err := os.MkdirAll(input.OutputDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}

		// Write upgraded values file
		outputPath := filepath.Join(input.OutputDir, "values-upgraded.yaml")
		if err := os.WriteFile(outputPath, []byte(upgradedYAML), 0644); err != nil {
			return nil, fmt.Errorf("failed to write upgraded values: %w", err)
		}

		output.OutputPath = outputPath
		slog.Debug("upgrade complete", "outputPath", outputPath)
	} else {
		slog.Debug("dry run - no files written")
	}

	return output, nil
}
