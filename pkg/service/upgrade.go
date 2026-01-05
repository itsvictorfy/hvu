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
	Chart         string
	Repository    string
	FromVersion   string
	ToVersion     string
	ValuesFile    string
	OutputDir     string
	DryRun        bool
	UpgradeImages bool // If true, automatically upgrade custom image tags
}

// UpgradeOutput contains the results of upgrade
type UpgradeOutput struct {
	Classification     *values.ClassificationResult
	UpgradedYAML       string
	OutputPath         string
	OldDefaultsCount   int
	NewDefaultsCount   int
	UserValuesCount    int
	CustomImageTags    []values.ImageChange // Detected custom image tags
	ImageTagsUpgraded  bool                 // Whether user chose to upgrade image tags
	PromptForImageTags bool                 // Whether to prompt user about image tags
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

	// Fetch old and new chart defaults in parallel
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

	// Parse user values
	slog.Debug("parsing user values", "file", input.ValuesFile)

	userValues, err := values.ParseFile(input.ValuesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user values: %w", err)
	}

	slog.Debug("parsed user values", "count", len(userValues))

	// Classify user values against old defaults
	slog.Debug("classifying user values")

	classification := values.Classify(userValues, oldDefaults)

	slog.Debug("classification complete",
		"customized", classification.Customized,
		"copiedDefault", classification.CopiedDefault,
		"unknown", classification.Unknown,
	)

	// Extract comments from new chart defaults
	slog.Debug("extracting comments from target chart")

	newComments := values.ExtractComments(newDefaultsYAML)
	slog.Debug("extracted comments", "count", len(newComments))

	// Merge values
	slog.Debug("generating upgraded values")

	upgradedValues := values.Merge(userValues, oldDefaults, newDefaults)

	// Detect custom image tags
	customImageTags := values.DetectCustomImageTags(userValues, oldDefaults, newDefaults)
	promptForImageTags := false
	imageTagsUpgraded := false

	if len(customImageTags) > 0 {
		slog.Debug("detected custom image tags", "count", len(customImageTags))

		if input.UpgradeImages {
			// Auto-upgrade image tags
			upgradedValues = values.ApplyImageUpgrades(upgradedValues, customImageTags)
			imageTagsUpgraded = true
			slog.Debug("applied image tag upgrades")
		} else {
			// Signal that CLI should prompt user
			promptForImageTags = true
		}
	}

	// Generate YAML output with comments from target chart
	upgradedYAML, err := upgradedValues.ToYAMLWithComments(newComments)
	if err != nil {
		return nil, fmt.Errorf("failed to generate YAML: %w", err)
	}

	output := &UpgradeOutput{
		Classification:     classification,
		UpgradedYAML:       upgradedYAML,
		OldDefaultsCount:   len(oldDefaults),
		NewDefaultsCount:   len(newDefaults),
		UserValuesCount:    len(userValues),
		CustomImageTags:    customImageTags,
		ImageTagsUpgraded:  imageTagsUpgraded,
		PromptForImageTags: promptForImageTags,
	}

	// Write output (unless dry run or prompting for image tags)
	if !input.DryRun && !promptForImageTags {
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
	} else if input.DryRun {
		slog.Debug("dry run - no files written")
	} else if promptForImageTags {
		slog.Debug("prompting for image tags - no files written yet")
	}

	return output, nil
}

// FinalizeUpgradeInput contains parameters for finalizing an upgrade after user prompt
type FinalizeUpgradeInput struct {
	OriginalOutput *UpgradeOutput
	ApplyUpgrades  bool   // Whether to apply image tag upgrades
	Chart          string // Chart name for filename
	ToVersion      string // Target version for filename
	OutputDir      string
	DryRun         bool
}

// FinalizeUpgrade applies the user's image tag decision and writes the final output
func FinalizeUpgrade(input *FinalizeUpgradeInput) (*UpgradeOutput, error) {
	output := input.OriginalOutput

	// If user chose to upgrade images, regenerate the YAML
	if input.ApplyUpgrades && len(output.CustomImageTags) > 0 {
		// Parse the current YAML to get values
		currentValues, err := values.ParseYAML(output.UpgradedYAML)
		if err != nil {
			return nil, fmt.Errorf("failed to parse current YAML: %w", err)
		}

		// Apply image upgrades
		upgradedValues := values.ApplyImageUpgrades(currentValues, output.CustomImageTags)

		// Regenerate YAML (without comments for simplicity in this path)
		upgradedYAML, err := upgradedValues.ToYAML()
		if err != nil {
			return nil, fmt.Errorf("failed to regenerate YAML: %w", err)
		}

		output.UpgradedYAML = upgradedYAML
		output.ImageTagsUpgraded = true
	}

	output.PromptForImageTags = false

	// Write output (unless dry run)
	if !input.DryRun {
		slog.Debug("writing output file", "dir", input.OutputDir)

		if err := os.MkdirAll(input.OutputDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}

		fileName := fmt.Sprintf("%s-%s-%s.yaml", input.Chart, input.ToVersion, time.Now().Format("2006-01-02-150405"))
		outputPath := filepath.Join(input.OutputDir, fileName)
		if err := os.WriteFile(outputPath, []byte(output.UpgradedYAML), 0644); err != nil {
			return nil, fmt.Errorf("failed to write upgraded values: %w", err)
		}

		output.OutputPath = outputPath
		slog.Debug("upgrade finalized", "outputPath", outputPath)
	}

	return output, nil
}
