package service

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

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

	// Step 1: Fetch old and new chart defaults in parallel
	slog.Debug("fetching chart defaults",
		"oldVersion", input.FromVersion,
		"newVersion", input.ToVersion,
	)

	var (
		oldDefaultsYAML, newDefaultsYAML string
		oldFetchErr, newFetchErr         error
		wg                               sync.WaitGroup
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		oldDefaultsYAML, oldFetchErr = helm.GetValuesFileByVersion(input.Repository, input.Chart, input.FromVersion)
	}()

	go func() {
		defer wg.Done()
		newDefaultsYAML, newFetchErr = helm.GetValuesFileByVersion(input.Repository, input.Chart, input.ToVersion)
	}()

	wg.Wait()

	// Check for fetch errors
	if oldFetchErr != nil {
		return nil, fmt.Errorf("failed to fetch old chart defaults: %w", oldFetchErr)
	}
	if newFetchErr != nil {
		return nil, fmt.Errorf("failed to fetch new chart defaults: %w", newFetchErr)
	}

	// Parse old defaults
	oldDefaults, err := values.ParseYAML(oldDefaultsYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse old chart defaults: %w", err)
	}
	slog.Debug("parsed old defaults", "count", len(oldDefaults))

	// Parse new defaults
	newDefaults, err := values.ParseYAML(newDefaultsYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse new chart defaults: %w", err)
	}
	slog.Debug("parsed new defaults", "count", len(newDefaults))

	// Step 2: Parse user values
	slog.Debug("parsing user values", "file", input.ValuesFile)

	userValues, err := values.ParseFile(input.ValuesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user values: %w", err)
	}

	slog.Debug("parsed user values", "count", len(userValues))

	// Step 3: Classify user values against old defaults
	slog.Debug("classifying user values")

	classification := values.Classify(userValues, oldDefaults)

	slog.Debug("classification complete",
		"customized", classification.Customized,
		"copiedDefault", classification.CopiedDefault,
		"unknown", classification.Unknown,
	)

	// Step 4: Extract comments from new chart defaults
	slog.Debug("extracting comments from target chart")

	newComments := values.ExtractComments(newDefaultsYAML)
	slog.Debug("extracted comments", "count", len(newComments))

	// Step 5: Merge values
	slog.Debug("generating upgraded values")

	upgradedValues := values.Merge(userValues, oldDefaults, newDefaults)

	// Generate YAML output with comments from target chart
	upgradedYAML, err := upgradedValues.ToYAMLWithComments(newComments)
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
		fileName := fmt.Sprintf("%s-%s-%s.yaml", input.Chart, input.ToVersion, time.Now().Format("2006-01-02-150405"))
		// Write upgraded values file
		outputPath := filepath.Join(input.OutputDir, fileName)
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
