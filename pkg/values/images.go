package values

import (
	"strings"
)

// ImageChange represents a detected change in image tag
type ImageChange struct {
	Path         string
	UserTag      string
	OldDefault   string
	NewDefault   string
	IsCustomized bool
}

// imageTagPatterns are common path suffixes that indicate image tags
var imageTagPatterns = []string{
	"::tag",
	"::image::tag",
}

// DetectCustomImageTags finds image tags where the user has customized the value
// and compares them against old and new defaults
func DetectCustomImageTags(userValues, oldDefaults, newDefaults Values) []ImageChange {
	var changes []ImageChange

	for path, userVal := range userValues {
		if !isImageTagPath(path) {
			continue
		}

		userTag, ok := userVal.(string)
		if !ok {
			continue
		}

		oldDefault, oldExists := oldDefaults[path]
		newDefault, newExists := newDefaults[path]

		if !oldExists || !newExists {
			continue
		}

		oldDefaultStr, oldOk := oldDefault.(string)
		newDefaultStr, newOk := newDefault.(string)

		if !oldOk || !newOk {
			continue
		}

		isCustomized := userTag != oldDefaultStr

		if isCustomized && newDefaultStr != oldDefaultStr {
			changes = append(changes, ImageChange{
				Path:         path,
				UserTag:      userTag,
				OldDefault:   oldDefaultStr,
				NewDefault:   newDefaultStr,
				IsCustomized: true,
			})
		}
	}

	return changes
}

// isImageTagPath checks if a path looks like an image tag path
func isImageTagPath(path string) bool {
	for _, pattern := range imageTagPatterns {
		if strings.HasSuffix(path, pattern) {
			return true
		}
	}
	return false
}

// ApplyImageUpgrades updates the values with new image tags for the specified changes
func ApplyImageUpgrades(values Values, upgrades []ImageChange) Values {
	result := make(Values)
	for k, v := range values {
		result[k] = v
	}

	for _, change := range upgrades {
		result[change.Path] = change.NewDefault
	}

	return result
}
