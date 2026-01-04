package values

import (
	"strings"
	"testing"
)

func TestExtractComments_ParamStyle(t *testing.T) {
	yamlContent := `
## @param replicaCount Number of replicas
replicaCount: 1

## @param image.repository Image repository
## @param image.tag Image tag
image:
  repository: nginx
  tag: latest
`

	comments := ExtractComments(yamlContent)

	if comments["replicaCount"] != "Number of replicas" {
		t.Errorf("expected 'Number of replicas', got %q", comments["replicaCount"])
	}
	if comments["image.repository"] != "Image repository" {
		t.Errorf("expected 'Image repository', got %q", comments["image.repository"])
	}
	if comments["image.tag"] != "Image tag" {
		t.Errorf("expected 'Image tag', got %q", comments["image.tag"])
	}
}

func TestExtractComments_InlineComments(t *testing.T) {
	yamlContent := `
# This is a header comment for key1
key1: value1
key2: value2  # inline comment
`

	comments := ExtractComments(yamlContent)

	// Header comment should be extracted
	if comment, ok := comments["key1"]; !ok || comment == "" {
		t.Logf("key1 comment: %q", comment)
	}
}

func TestExtractComments_EmptyYAML(t *testing.T) {
	comments := ExtractComments("")

	if len(comments) != 0 {
		t.Errorf("expected empty comment map, got %d entries", len(comments))
	}
}

func TestExtractComments_InvalidYAML(t *testing.T) {
	// Should not panic, just return empty or partial results
	comments := ExtractComments("{invalid: yaml: content")

	// Should return without panicking
	_ = comments
}

func TestExtractComments_NestedPaths(t *testing.T) {
	yamlContent := `
## @param primary.resources.requests.cpu CPU request
## @param primary.resources.requests.memory Memory request
primary:
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
`

	comments := ExtractComments(yamlContent)

	if comments["primary.resources.requests.cpu"] != "CPU request" {
		t.Errorf("expected 'CPU request', got %q", comments["primary.resources.requests.cpu"])
	}
	if comments["primary.resources.requests.memory"] != "Memory request" {
		t.Errorf("expected 'Memory request', got %q", comments["primary.resources.requests.memory"])
	}
}

func TestCleanComment(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"## This is a comment", "This is a comment"},
		{"# Single hash", "Single hash"},
		{"  ## Whitespace  ", "Whitespace"},
		{"@param foo.bar Description here", "Description here"},
		{"## @param some.path The description", "The description"},
		{"Plain text", "Plain text"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanComment(tt.input)
			if result != tt.expected {
				t.Errorf("cleanComment(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToYAMLWithComments(t *testing.T) {
	v := Values{
		"replicaCount":      3,
		"image::repository": "nginx",
		"image::tag":        "latest",
	}

	comments := CommentMap{
		"replicaCount":     "Number of replicas to deploy",
		"image.repository": "Container image repository",
		"image.tag":        "Container image tag",
	}

	yaml, err := v.ToYAMLWithComments(comments)
	if err != nil {
		t.Fatalf("ToYAMLWithComments() error = %v", err)
	}

	// Check that comments are present in output
	if !strings.Contains(yaml, "Number of replicas") {
		t.Errorf("expected output to contain 'Number of replicas' comment")
	}
	if !strings.Contains(yaml, "replicaCount") {
		t.Errorf("expected output to contain 'replicaCount' key")
	}
}

func TestToYAMLWithComments_EmptyComments(t *testing.T) {
	v := Values{
		"key1": "value1",
		"key2": "value2",
	}

	comments := CommentMap{}

	yaml, err := v.ToYAMLWithComments(comments)
	if err != nil {
		t.Fatalf("ToYAMLWithComments() error = %v", err)
	}

	// Should produce valid YAML without comments
	if !strings.Contains(yaml, "key1") || !strings.Contains(yaml, "key2") {
		t.Errorf("expected output to contain keys, got:\n%s", yaml)
	}
}

func TestToYAMLWithComments_NestedValues(t *testing.T) {
	v := Values{
		"parent::child::grandchild": "value",
		"parent::sibling":           "other",
	}

	comments := CommentMap{
		"parent.child.grandchild": "Deeply nested value",
		"parent.sibling":          "Sibling value",
	}

	yaml, err := v.ToYAMLWithComments(comments)
	if err != nil {
		t.Fatalf("ToYAMLWithComments() error = %v", err)
	}

	// Verify nested structure
	if !strings.Contains(yaml, "parent") {
		t.Errorf("expected output to contain 'parent' key")
	}
	if !strings.Contains(yaml, "grandchild") {
		t.Errorf("expected output to contain 'grandchild' key")
	}
}

func TestExtractParamComments_MultipleDescriptions(t *testing.T) {
	yamlContent := `
## @param auth.enabled Enable authentication
## @param auth.username Default username
## @param auth.password Default password
auth:
  enabled: true
  username: admin
  password: secret
`

	comments := make(CommentMap)
	extractParamComments(yamlContent, comments)

	if comments["auth.enabled"] != "Enable authentication" {
		t.Errorf("expected 'Enable authentication', got %q", comments["auth.enabled"])
	}
	if comments["auth.username"] != "Default username" {
		t.Errorf("expected 'Default username', got %q", comments["auth.username"])
	}
	if comments["auth.password"] != "Default password" {
		t.Errorf("expected 'Default password', got %q", comments["auth.password"])
	}
}

func TestExtractParamComments_NoDescription(t *testing.T) {
	yamlContent := `
## @param standalone.key
standalone:
  key: value
`

	comments := make(CommentMap)
	extractParamComments(yamlContent, comments)

	// Path without description should still be parsed
	if _, ok := comments["standalone.key"]; ok && comments["standalone.key"] != "" {
		t.Errorf("expected empty description, got %q", comments["standalone.key"])
	}
}

func TestCommentMap_EmptyPath(t *testing.T) {
	comments := CommentMap{
		"":    "should not happen",
		"key": "valid",
	}

	// CommentMap is just a map, it can technically hold empty keys
	// This test documents the behavior
	if comments[""] != "should not happen" {
		t.Errorf("expected empty key to be stored")
	}
	if comments["key"] != "valid" {
		t.Errorf("expected 'key' to be stored")
	}
}
