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
func escapeKeyDots(key string) string {
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
		escapedKey := escapeKeyDots(key)
		fullKey := escapedKey
		if prefix != "" {
			fullKey = prefix + pathSeparator + escapedKey
		}

		switch v := value.(type) {
		case map[string]interface{}:
			if len(v) == 0 {
				result[fullKey] = map[string]interface{}{}
			} else {
				flatten(fullKey, v, result)
			}
		default:
			result[fullKey] = value
		}
	}
}

// Unflatten converts a flat map back to a nested structure
// Uses "::" as the path separator and unescapes key names to restore dots
func Unflatten(flat Values) map[string]interface{} {
	result := make(map[string]interface{})

	paths := make([]string, 0, len(flat))
	for path := range flat {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		depthI := strings.Count(paths[i], pathSeparator)
		depthJ := strings.Count(paths[j], pathSeparator)
		if depthI != depthJ {
			return depthI > depthJ // Deeper paths first
		}
		return paths[i] < paths[j]
	})

	for _, path := range paths {
		value := flat[path]
		parts := strings.Split(path, pathSeparator)
		current := result

		for i, part := range parts {
			originalKey := unescapeKeyDots(part)

			if i == len(parts)-1 {
				if _, exists := current[originalKey]; !exists {
					current[originalKey] = value
				} else if emptyMap, ok := value.(map[string]interface{}); ok && len(emptyMap) == 0 {
					continue
				} else {
					current[originalKey] = value
				}
			} else {
				if _, exists := current[originalKey]; !exists {
					current[originalKey] = make(map[string]interface{})
				}
				nested, ok := current[originalKey].(map[string]interface{})
				if !ok {
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

	exactMatches := 0
	parentEmptyMapMatches := 0

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
			// Key doesn't exist in defaults - check if parent is an empty map or has children
			if parentDefault := findParentEmptyMap(path, defaultValues); parentDefault != "" {
				entry.DefaultValue = nil
				entry.Classification = Customized
				result.Customized++
				parentEmptyMapMatches++
				slog.Debug("customized value (parent was empty map)",
					"path", path,
					"userValue", FormatValue(userVal),
					"parentPath", parentDefault,
				)
			} else if parentWithChildren := findParentWithChildren(path, defaultValues); parentWithChildren != "" {
				// Parent exists with different children - user is customizing the map contents
				entry.DefaultValue = nil
				entry.Classification = Customized
				result.Customized++
				slog.Debug("customized value (parent has children with different keys)",
					"path", path,
					"userValue", FormatValue(userVal),
					"parentPath", parentWithChildren,
				)
			} else {
				entry.Classification = Unknown
				result.Unknown++
				slog.Debug("unknown value",
					"path", path,
					"userValue", FormatValue(userVal),
					"reason", "not in defaults and no parent found",
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
func findParentEmptyMap(path string, defaults Values) string {
	parts := strings.Split(path, pathSeparator)

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

// findParentWithChildren checks if any parent path of the given key has children in defaults.
// This detects when a user customizes a map with different keys than the default
func findParentWithChildren(path string, defaults Values) string {
	parts := strings.Split(path, pathSeparator)

	// Check each parent level from most specific to least specific
	for i := len(parts) - 1; i > 0; i-- {
		parentPath := strings.Join(parts[:i], pathSeparator)
		parentPrefix := parentPath + pathSeparator

		// Look for any key in defaults that starts with this parent prefix
		for defaultPath := range defaults {
			if strings.HasPrefix(defaultPath, parentPrefix) {
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
func Merge(userValues, oldDefaults, newDefaults Values) Values {
	result := make(Values)

	customizedParents := findCustomizedParentMaps(userValues, oldDefaults)

	for path, value := range newDefaults {
		if isUnderCustomizedParent(path, customizedParents) {
			continue
		}
		result[path] = value
	}

	for path, userVal := range userValues {
		oldDefault, existsInOld := oldDefaults[path]

		if !existsInOld || !ValuesEqual(userVal, oldDefault) {
			result[path] = userVal
		}
	}

	return result
}

// findCustomizedParentMaps finds parent paths where the user has customized children
// with different keys than the old defaults. This indicates the user wants to replace
// the entire map, not merge with it.
func findCustomizedParentMaps(userValues, oldDefaults Values) map[string]bool {
	customizedParents := make(map[string]bool)

	for userPath := range userValues {
		// Skip if exact path exists in old defaults
		if _, exists := oldDefaults[userPath]; exists {
			continue
		}

		// Get the immediate parent (one level up)
		parts := strings.Split(userPath, pathSeparator)
		if len(parts) < 2 {
			continue
		}

		parentPath := strings.Join(parts[:len(parts)-1], pathSeparator)
		parentPrefix := parentPath + pathSeparator

		if val, exists := oldDefaults[parentPath]; exists {
			if emptyMap, ok := val.(map[string]interface{}); ok && len(emptyMap) == 0 {
				continue // Empty map case - not a "replacement", just adding to empty
			}
		}

		for oldPath := range oldDefaults {
			if strings.HasPrefix(oldPath, parentPrefix) && oldPath != userPath {
				customizedParents[parentPath] = true
				break
			}
		}
	}

	return customizedParents
}

// isUnderCustomizedParent checks if a path is a child of any customized parent
func isUnderCustomizedParent(path string, customizedParents map[string]bool) bool {
	parts := strings.Split(path, pathSeparator)
	for i := len(parts) - 1; i > 0; i-- {
		parentPath := strings.Join(parts[:i], pathSeparator)
		if customizedParents[parentPath] {
			return true
		}
	}
	return false
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
func PathToDisplayFormat(path string) string {
	return strings.ReplaceAll(path, pathSeparator, ".")
}
