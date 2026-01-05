package values

import (
	"bufio"
	"fmt"
	"log/slog"
	"strings"

	"gopkg.in/yaml.v3"
)

// CommentMap stores comments for each path in a values file
type CommentMap map[string]string

// ExtractComments extracts comments from YAML content and associates them with their paths
// This extracts comments from the TARGET chart version to document values in the upgraded file
func ExtractComments(yamlContent string) CommentMap {
	comments := make(CommentMap)

	// Parse with yaml.v3 to get comment nodes
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &root); err != nil {
		slog.Warn("failed to parse YAML for comment extraction", "error", err)
		return comments
	}

	// Extract comments from the node tree
	extractCommentsFromNode(&root, "", comments)

	// Also extract @param style comments from the raw content
	// This is the primary way Helm charts document their values
	extractParamComments(yamlContent, comments)

	return comments
}

// extractCommentsFromNode recursively extracts comments from yaml.Node tree
func extractCommentsFromNode(node *yaml.Node, prefix string, comments CommentMap) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.DocumentNode:
		// Process document content
		for _, child := range node.Content {
			extractCommentsFromNode(child, prefix, comments)
		}

	case yaml.MappingNode:
		// Process key-value pairs
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			key := keyNode.Value
			fullPath := key
			if prefix != "" {
				fullPath = prefix + "." + key
			}

			// Extract head comment (comment before the key)
			if keyNode.HeadComment != "" {
				comments[fullPath] = cleanComment(keyNode.HeadComment)
			}

			// Extract line comment (comment on the same line)
			if keyNode.LineComment != "" {
				if existing, ok := comments[fullPath]; ok {
					comments[fullPath] = existing + " " + cleanComment(keyNode.LineComment)
				} else {
					comments[fullPath] = cleanComment(keyNode.LineComment)
				}
			}
			extractCommentsFromNode(valueNode, fullPath, comments)
		}

	case yaml.SequenceNode:
		for _, child := range node.Content {
			extractCommentsFromNode(child, prefix, comments)
		}
	}
}

// extractParamComments extracts @param style comments from raw YAML content
// Format: ## @param path.to.value Description of the value
func extractParamComments(yamlContent string, comments CommentMap) {
	scanner := bufio.NewScanner(strings.NewReader(yamlContent))

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Check if it's a @param comment
		if strings.HasPrefix(trimmed, "## @param ") {
			// Extract path and description
			parts := strings.SplitN(strings.TrimPrefix(trimmed, "## @param "), " ", 2)
			if len(parts) >= 1 {
				path := parts[0]
				description := ""
				if len(parts) == 2 {
					description = parts[1]
				}

				// Store the description for this path
				if description != "" {
					if existing, ok := comments[path]; ok && existing != "" {
						// Don't add if existing already contains this description
						if !strings.Contains(existing, description) {
							comments[path] = existing + " | " + description
						}
					} else {
						comments[path] = description
					}
				}
			}
		}
	}
}

// cleanComment removes comment markers and extra whitespace
func cleanComment(comment string) string {
	// Remove leading/trailing whitespace
	comment = strings.TrimSpace(comment)

	// Remove leading ## or #
	comment = strings.TrimPrefix(comment, "##")
	comment = strings.TrimPrefix(comment, "#")

	// Remove leading/trailing whitespace again
	comment = strings.TrimSpace(comment)

	// If comment starts with @param, extract just the description part
	// Format: @param path.to.value Description here
	if strings.HasPrefix(comment, "@param ") {
		parts := strings.SplitN(strings.TrimPrefix(comment, "@param "), " ", 2)
		if len(parts) == 2 {
			comment = parts[1]
		}
	}

	return comment
}

// ToYAMLWithComments converts Values to YAML string with comments from the provided CommentMap
func (v Values) ToYAMLWithComments(comments CommentMap) (string, error) {
	// Create nested structure
	nested := Unflatten(v)

	// Convert to yaml.Node tree to allow comment injection
	var node yaml.Node
	node.Kind = yaml.DocumentNode

	// Encode the nested structure to a node
	contentNode := &yaml.Node{}
	if err := contentNode.Encode(nested); err != nil {
		return "", fmt.Errorf("failed to encode to node: %w", err)
	}

	// Attach comments to the node tree
	attachCommentsToNode(contentNode, "", comments)

	node.Content = append(node.Content, contentNode)

	// Marshal to YAML with comments
	out, err := yaml.Marshal(&node)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML with comments: %w", err)
	}

	return string(out), nil
}

// attachCommentsToNode recursively attaches comments to yaml.Node tree
func attachCommentsToNode(node *yaml.Node, prefix string, comments CommentMap) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		// Process key-value pairs
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			key := keyNode.Value
			fullPath := key
			if prefix != "" {
				fullPath = prefix + "." + key
			}

			// Attach comment if available from target chart
			if comment, ok := comments[fullPath]; ok && comment != "" {
				// Format as Helm-style comment
				keyNode.HeadComment = "## " + comment
			}

			// Recurse into nested structures
			attachCommentsToNode(valueNode, fullPath, comments)
		}

	case yaml.SequenceNode:
		// Process array elements
		for _, child := range node.Content {
			attachCommentsToNode(child, prefix, comments)
		}
	}
}
