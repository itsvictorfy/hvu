package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/itsvictorfy/hvu/pkg/helm"
	"github.com/itsvictorfy/hvu/pkg/values"
)

// ClassifyInput contains input parameters for classification
type ClassifyInput struct {
	Chart      string
	Repository string
	Version    string
	ValuesFile string
}

// ClassifyOutput contains the results of classification
type ClassifyOutput struct {
	Result        *values.ClassificationResult
	DefaultsCount int
	UserCount     int
}

// Classify runs the classification logic
func Classify(input *ClassifyInput) (*ClassifyOutput, error) {
	slog.Debug("starting classification",
		"chart", input.Chart,
		"repository", input.Repository,
		"version", input.Version,
		"valuesFile", input.ValuesFile,
	)

	// Validate values file exists
	if _, err := os.Stat(input.ValuesFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("values file not found: %s", input.ValuesFile)
	}

	// Fetch chart defaults
	slog.Debug("fetching default values", "chart", input.Chart, "version", input.Version)

	defaultsYAML, err := helm.GetValuesFileByVersion(input.Repository, input.Chart, input.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chart defaults: %w", err)
	}

	// Parse default values
	defaultValues, err := values.ParseYAML(defaultsYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse chart defaults: %w", err)
	}

	slog.Debug("parsed default values", "count", len(defaultValues))

	// Count empty maps for debugging (only when verbose logging is enabled)
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		emptyMaps := 0
		for k, v := range defaultValues {
			if m, ok := v.(map[string]interface{}); ok && len(m) == 0 {
				emptyMaps++
				if emptyMaps <= 5 {
					slog.Debug("empty map in defaults", "key", k)
				}
			}
		}
		if emptyMaps > 0 {
			slog.Debug("total empty maps in defaults", "count", emptyMaps)
		}
	}

	// Parse user values
	userValues, err := values.ParseFile(input.ValuesFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user values: %w", err)
	}

	slog.Debug("parsed user values", "count", len(userValues))
	slog.Debug("starting classification process")

	// Classify values
	result := values.Classify(userValues, defaultValues)

	slog.Debug("classification complete",
		"customized", result.Customized,
		"copiedDefault", result.CopiedDefault,
		"unknown", result.Unknown,
		"total", result.Total,
	)

	return &ClassifyOutput{
		Result:        result,
		DefaultsCount: len(defaultValues),
		UserCount:     len(userValues),
	}, nil
}
