package values

import (
	"testing"
)

func TestIsImageTagPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"image::tag", true},
		{"controller::image::tag", true},
		{"webhook::image::tag", true},
		{"primary::image::tag", true},
		{"image::repository", false},
		{"image::pullPolicy", false},
		{"replicaCount", false},
		{"name", false},
		{"tags::enabled", false}, // "tags" is not "tag"
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isImageTagPath(tt.path)
			if result != tt.expected {
				t.Errorf("isImageTagPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestDetectCustomImageTags_NoCustomTags(t *testing.T) {
	// User values match old defaults - no custom tags
	userValues := Values{
		"image::tag":        "1.0.0",
		"image::repository": "nginx",
	}
	oldDefaults := Values{
		"image::tag":        "1.0.0",
		"image::repository": "nginx",
	}
	newDefaults := Values{
		"image::tag":        "2.0.0",
		"image::repository": "nginx",
	}

	changes := DetectCustomImageTags(userValues, oldDefaults, newDefaults)

	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
	}
}

func TestDetectCustomImageTags_WithCustomTag(t *testing.T) {
	// User has customized image tag
	userValues := Values{
		"image::tag":        "1.5.0", // customized
		"image::repository": "nginx",
	}
	oldDefaults := Values{
		"image::tag":        "1.0.0",
		"image::repository": "nginx",
	}
	newDefaults := Values{
		"image::tag":        "2.0.0",
		"image::repository": "nginx",
	}

	changes := DetectCustomImageTags(userValues, oldDefaults, newDefaults)

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}

	change := changes[0]
	if change.Path != "image::tag" {
		t.Errorf("expected path 'image::tag', got %q", change.Path)
	}
	if change.UserTag != "1.5.0" {
		t.Errorf("expected UserTag '1.5.0', got %q", change.UserTag)
	}
	if change.OldDefault != "1.0.0" {
		t.Errorf("expected OldDefault '1.0.0', got %q", change.OldDefault)
	}
	if change.NewDefault != "2.0.0" {
		t.Errorf("expected NewDefault '2.0.0', got %q", change.NewDefault)
	}
	if !change.IsCustomized {
		t.Error("expected IsCustomized to be true")
	}
}

func TestDetectCustomImageTags_MultipleCustomTags(t *testing.T) {
	// Multiple custom image tags
	userValues := Values{
		"image::tag":             "1.5.0",
		"controller::image::tag": "v2.1.0",
		"webhook::image::tag":    "0.9.0", // matches old default
	}
	oldDefaults := Values{
		"image::tag":             "1.0.0",
		"controller::image::tag": "v2.0.0",
		"webhook::image::tag":    "0.9.0",
	}
	newDefaults := Values{
		"image::tag":             "2.0.0",
		"controller::image::tag": "v3.0.0",
		"webhook::image::tag":    "1.0.0",
	}

	changes := DetectCustomImageTags(userValues, oldDefaults, newDefaults)

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}

	// Verify we found the right paths
	paths := make(map[string]bool)
	for _, c := range changes {
		paths[c.Path] = true
	}

	if !paths["image::tag"] {
		t.Error("expected image::tag to be detected")
	}
	if !paths["controller::image::tag"] {
		t.Error("expected controller::image::tag to be detected")
	}
	if paths["webhook::image::tag"] {
		t.Error("webhook::image::tag should NOT be detected (matches old default)")
	}
}

func TestDetectCustomImageTags_NoChangeInDefaults(t *testing.T) {
	// User customized, but old and new defaults are the same
	userValues := Values{
		"image::tag": "custom",
	}
	oldDefaults := Values{
		"image::tag": "1.0.0",
	}
	newDefaults := Values{
		"image::tag": "1.0.0", // same as old
	}

	changes := DetectCustomImageTags(userValues, oldDefaults, newDefaults)

	// Should not report since there's nothing to upgrade to
	if len(changes) != 0 {
		t.Errorf("expected 0 changes when defaults unchanged, got %d", len(changes))
	}
}

func TestDetectCustomImageTags_PathNotInDefaults(t *testing.T) {
	// Image tag path doesn't exist in defaults
	userValues := Values{
		"custom::image::tag": "1.0.0",
	}
	oldDefaults := Values{
		"other::key": "value",
	}
	newDefaults := Values{
		"other::key": "newvalue",
	}

	changes := DetectCustomImageTags(userValues, oldDefaults, newDefaults)

	if len(changes) != 0 {
		t.Errorf("expected 0 changes when path not in defaults, got %d", len(changes))
	}
}

func TestDetectCustomImageTags_NonStringValues(t *testing.T) {
	// Handle non-string values gracefully
	userValues := Values{
		"image::tag": 123, // not a string
	}
	oldDefaults := Values{
		"image::tag": "1.0.0",
	}
	newDefaults := Values{
		"image::tag": "2.0.0",
	}

	changes := DetectCustomImageTags(userValues, oldDefaults, newDefaults)

	// Should skip non-string values
	if len(changes) != 0 {
		t.Errorf("expected 0 changes for non-string tag, got %d", len(changes))
	}
}

func TestApplyImageUpgrades(t *testing.T) {
	values := Values{
		"image::tag":             "1.5.0",
		"image::repository":      "nginx",
		"controller::image::tag": "v2.1.0",
		"replicaCount":           3,
	}

	upgrades := []ImageChange{
		{
			Path:       "image::tag",
			UserTag:    "1.5.0",
			NewDefault: "2.0.0",
		},
		{
			Path:       "controller::image::tag",
			UserTag:    "v2.1.0",
			NewDefault: "v3.0.0",
		},
	}

	result := ApplyImageUpgrades(values, upgrades)

	// Check upgraded values
	if result["image::tag"] != "2.0.0" {
		t.Errorf("expected image::tag to be upgraded to '2.0.0', got %v", result["image::tag"])
	}
	if result["controller::image::tag"] != "v3.0.0" {
		t.Errorf("expected controller::image::tag to be upgraded to 'v3.0.0', got %v", result["controller::image::tag"])
	}

	// Check non-upgraded values remain
	if result["image::repository"] != "nginx" {
		t.Errorf("expected image::repository to remain 'nginx', got %v", result["image::repository"])
	}
	if result["replicaCount"] != 3 {
		t.Errorf("expected replicaCount to remain 3, got %v", result["replicaCount"])
	}

	// Check original values are not modified
	if values["image::tag"] != "1.5.0" {
		t.Error("original values should not be modified")
	}
}

func TestApplyImageUpgrades_EmptyUpgrades(t *testing.T) {
	values := Values{
		"image::tag": "1.0.0",
	}

	result := ApplyImageUpgrades(values, []ImageChange{})

	if result["image::tag"] != "1.0.0" {
		t.Errorf("expected image::tag to remain '1.0.0', got %v", result["image::tag"])
	}
}

func TestApplyImageUpgrades_NilUpgrades(t *testing.T) {
	values := Values{
		"image::tag": "1.0.0",
	}

	result := ApplyImageUpgrades(values, nil)

	if result["image::tag"] != "1.0.0" {
		t.Errorf("expected image::tag to remain '1.0.0', got %v", result["image::tag"])
	}
}
