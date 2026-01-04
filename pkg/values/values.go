package values

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Classification represents how a value was classified
type Classification string

const (
	Customized    Classification = "CUSTOMIZED"     // Value differs from default (user change)
	CopiedDefault Classification = "COPIED_DEFAULT" // Value matches default
	Unknown       Classification = "UNKNOWN"        // Not in chart defaults (may be obsolete or custom)
)

// ClassifiedValue holds a value and its classification
type ClassifiedValue struct {
	Path           string      // Dot-separated path (e.g., "image.repository")
	UserValue      interface{} // Value from user's values file
	DefaultValue   interface{} // Value from chart defaults (nil if Unknown)
	Classification Classification
}

// ClassificationResult holds the complete classification results
type ClassificationResult struct {
	Entries       []ClassifiedValue
	Customized    int
	CopiedDefault int
	Unknown       int
	Total         int
}

// Values represents a parsed values file as a flat key-value map
type Values map[string]interface{}

// ParseYAML parses YAML content into a Values map
func ParseYAML(content string) (Values, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	return Flatten(data), nil
}

// ParseFile reads and parses a YAML file
func ParseFile(path string) (Values, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return ParseYAML(string(content))
}

// pathSeparator is used to separate path components in flattened keys
const pathSeparator = "::"

// escapeKeyDots escapes dots in a single key name so they don't conflict with path separators
// This is needed because YAML keys can contain dots (e.g., "grafana.ini", "datasources.yaml")
func escapeKeyDots(key string) string {
	// Escape existing path separators first (to handle edge cases)
	key = strings.ReplaceAll(key, pathSeparator, "\\"+pathSeparator)
	return key
}

// unescapeKeyDots reverses the escaping done by escapeKeyDots
func unescapeKeyDots(key string) string {
	return strings.ReplaceAll(key, "\\"+pathSeparator, pathSeparator)
}

// Flatten converts a nested map to a flat map with path-separated keys
// Uses "::" as the path separator to avoid conflicts with dots in YAML key names
func Flatten(data map[string]interface{}) Values {
	result := make(Values)
	flatten("", data, result)
	return result
}

func flatten(prefix string, data map[string]interface{}, result Values) {
	for key, value := range data {
		// Escape the key to preserve dots in key names
		escapedKey := escapeKeyDots(key)
		fullKey := escapedKey
		if prefix != "" {
			fullKey = prefix + pathSeparator + escapedKey
		}

		switch v := value.(type) {
		case map[string]interface{}:
			if len(v) == 0 {
				// Store empty maps as a marker value so they're preserved in the flattened structure
				// This is important for comparing against user values that add keys to previously empty maps
				result[fullKey] = map[string]interface{}{}
			} else {
				// Recurse into non-empty nested maps
				flatten(fullKey, v, result)
			}
		default:
			// Store leaf values
			result[fullKey] = value
		}
	}
}

// Unflatten converts a flat map back to a nested structure
// Uses "::" as the path separator and unescapes key names to restore dots
func Unflatten(flat Values) map[string]interface{} {
	result := make(map[string]interface{})

	// Get all paths and sort them by length (descending) and then alphabetically
	// This ensures child paths are processed before parent paths
	// e.g., "pdb::create" is processed before "pdb: {}"
	paths := make([]string, 0, len(flat))
	for path := range flat {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		// First sort by depth (number of separators) in descending order
		depthI := strings.Count(paths[i], pathSeparator)
		depthJ := strings.Count(paths[j], pathSeparator)
		if depthI != depthJ {
			return depthI > depthJ // Deeper paths first
		}
		// If same depth, sort alphabetically
		return paths[i] < paths[j]
	})

	for _, path := range paths {
		value := flat[path]
		parts := strings.Split(path, pathSeparator)
		current := result

		for i, part := range parts {
			// Unescape the key to restore original dots in key names
			originalKey := unescapeKeyDots(part)

			if i == len(parts)-1 {
				// Last part - set the value
				// Only set if it doesn't already exist (child was already set)
				if _, exists := current[originalKey]; !exists {
					current[originalKey] = value
				} else if emptyMap, ok := value.(map[string]interface{}); ok && len(emptyMap) == 0 {
					// If the value we're trying to set is an empty map and something already exists,
					// don't overwrite it (the existing content is from child paths)
					// This handles the case where "pdb: {}" tries to overwrite "pdb: {create: true}"
					continue
				} else {
					// Otherwise set the value (normal case)
					current[originalKey] = value
				}
			} else {
				// Create nested map if needed
				if _, exists := current[originalKey]; !exists {
					current[originalKey] = make(map[string]interface{})
				}
				// Safe type assertion to handle potential path conflicts
				nested, ok := current[originalKey].(map[string]interface{})
				if !ok {
					// Path conflict: existing value is not a map, create new map
					current[originalKey] = make(map[string]interface{})
					nested = current[originalKey].(map[string]interface{})
				}
				current = nested
			}
		}
	}

	return result
}

// Classify compares user values against defaults and classifies each key
func Classify(userValues, defaultValues Values) *ClassificationResult {
	result := &ClassificationResult{
		Entries: make([]ClassifiedValue, 0),
	}

	// Track some stats for logging
	exactMatches := 0
	parentEmptyMapMatches := 0

	// Process all user values
	for path, userVal := range userValues {
		entry := ClassifiedValue{
			Path:      path,
			UserValue: userVal,
		}

		if defaultVal, exists := defaultValues[path]; exists {
			// Exact path exists in defaults
			entry.DefaultValue = defaultVal
			if ValuesEqual(userVal, defaultVal) {
				entry.Classification = CopiedDefault
				result.CopiedDefault++
				exactMatches++
			} else {
				entry.Classification = Customized
				result.Customized++
				slog.Debug("customized value",
					"path", path,
					"userValue", FormatValue(userVal),
					"defaultValue", FormatValue(defaultVal),
				)
			}
		} else {
			// Key doesn't exist in defaults - but check if parent is an empty map
			// e.g., user has "primary.nodeSelector.workload-type" but defaults only has "primary.nodeSelector: {}"
			if parentDefault := findParentEmptyMap(path, defaultValues); parentDefault != "" {
				// Parent exists as empty map in defaults, user is adding content to it
				entry.DefaultValue = nil // Parent was empty map
				entry.Classification = Customized
				result.Customized++
				parentEmptyMapMatches++
				slog.Debug("customized value (parent was empty map)",
					"path", path,
					"userValue", FormatValue(userVal),
					"parentPath", parentDefault,
				)
			} else {
				entry.Classification = Unknown
				result.Unknown++
				slog.Debug("unknown value",
					"path", path,
					"userValue", FormatValue(userVal),
					"reason", "not in defaults and no parent empty map found",
				)
			}
		}

		result.Entries = append(result.Entries, entry)
		result.Total++
	}

	slog.Debug("classification complete",
		"exactMatches", exactMatches,
		"customizedFromDefault", result.Customized-parentEmptyMapMatches,
		"customizedFromEmptyMap", parentEmptyMapMatches,
		"unknown", result.Unknown,
		"total", result.Total,
	)

	// Sort entries by path for consistent output
	sort.Slice(result.Entries, func(i, j int) bool {
		return result.Entries[i].Path < result.Entries[j].Path
	})

	return result
}

// FormatValue formats a value for display, truncating long values
func FormatValue(v interface{}) string {
	s := fmt.Sprintf("%v", v)
	if len(s) > 100 {
		return s[:100] + "..."
	}
	return s
}

// findParentEmptyMap checks if any parent path of the given key is an empty map in defaults
// e.g., for "primary::nodeSelector::workload-type", check if "primary::nodeSelector" exists as empty map
func findParentEmptyMap(path string, defaults Values) string {
	parts := strings.Split(path, pathSeparator)

	// Check each parent level from most specific to least specific
	for i := len(parts) - 1; i > 0; i-- {
		parentPath := strings.Join(parts[:i], pathSeparator)
		if val, exists := defaults[parentPath]; exists {
			// Check if this parent is an empty map
			if emptyMap, ok := val.(map[string]interface{}); ok && len(emptyMap) == 0 {
				return parentPath
			}
		}
	}

	return ""
}

// ValuesEqual checks if two values are equal, handling various types
func ValuesEqual(a, b interface{}) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Use reflect.DeepEqual for complex comparisons
	return reflect.DeepEqual(a, b)
}

// Merge creates an upgraded values file by:
// 1. Starting with the new chart defaults
// 2. Overlaying user customizations (values that differ from old defaults)
func Merge(userValues, oldDefaults, newDefaults Values) Values {
	result := make(Values)

	// Start with new defaults
	for path, value := range newDefaults {
		result[path] = value
	}

	// Overlay user customizations
	for path, userVal := range userValues {
		oldDefault, existsInOld := oldDefaults[path]

		// If the value was customized (differs from old default), keep user's value
		if !existsInOld || !ValuesEqual(userVal, oldDefault) {
			result[path] = userVal
		}
		// If it matches old default, we already have new default in result
	}

	return result
}

// ToYAML converts Values back to YAML string
func (v Values) ToYAML() (string, error) {
	nested := Unflatten(v)
	out, err := yaml.Marshal(nested)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return string(out), nil
}

// GetPaths returns all paths in sorted order
func (v Values) GetPaths() []string {
	paths := make([]string, 0, len(v))
	for path := range v {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// PathToDisplayFormat converts an internal path (using "::" separator) to
// a human-readable format using "." for display purposes.
// This handles the conversion correctly, preserving dots that are part of key names.
func PathToDisplayFormat(path string) string {
	return strings.ReplaceAll(path, pathSeparator, ".")
}
