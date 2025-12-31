package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/itsvictorfy/hvu/pkg/values"
)

// testDataDir returns the path to the test-data directory
func testDataDir() string {
	return filepath.Join("..", "test-data")
}

// loadTestData loads a YAML file from test-data directory
func loadTestData(t *testing.T, filename string) values.Values {
	t.Helper()
	path := filepath.Join(testDataDir(), filename)
	v, err := values.ParseFile(path)
	if err != nil {
		t.Fatalf("failed to load %s: %v", filename, err)
	}
	return v
}

func TestScenario_AuthCustomized(t *testing.T) {
	// Skip if test-data not available
	if _, err := os.Stat(filepath.Join(testDataDir(), "defaults-v1.yaml")); os.IsNotExist(err) {
		t.Skip("test-data not available")
	}

	defaults := loadTestData(t, "defaults-v1.yaml")
	userValues := loadTestData(t, "scenario-auth-customized.yaml")

	result := values.Classify(userValues, defaults)

	// Should have 4 customized auth keys
	if result.Customized != 4 {
		t.Errorf("expected 4 customized keys, got %d", result.Customized)
		for _, e := range result.Entries {
			if e.Classification == values.Customized {
				t.Logf("  CUSTOMIZED: %s", e.Path)
			}
		}
	}

	// Should have no unknown keys
	if result.Unknown != 0 {
		t.Errorf("expected 0 unknown keys, got %d", result.Unknown)
		for _, e := range result.Entries {
			if e.Classification == values.Unknown {
				t.Logf("  UNKNOWN: %s = %v", e.Path, e.UserValue)
			}
		}
	}

	// Verify specific customized keys
	customizedPaths := make(map[string]bool)
	for _, e := range result.Entries {
		if e.Classification == values.Customized {
			customizedPaths[e.Path] = true
		}
	}

	expectedCustomized := []string{
		"global.postgresql.auth.existingSecret",
		"global.postgresql.auth.secretKeys.adminPasswordKey",
		"global.postgresql.auth.secretKeys.userPasswordKey",
		"global.postgresql.auth.secretKeys.replicationPasswordKey",
	}

	for _, path := range expectedCustomized {
		if !customizedPaths[path] {
			t.Errorf("expected %s to be customized", path)
		}
	}
}

func TestScenario_NodeSelectors(t *testing.T) {
	if _, err := os.Stat(filepath.Join(testDataDir(), "defaults-v1.yaml")); os.IsNotExist(err) {
		t.Skip("test-data not available")
	}

	defaults := loadTestData(t, "defaults-v1.yaml")
	userValues := loadTestData(t, "scenario-node-selectors.yaml")

	result := values.Classify(userValues, defaults)

	// Should have 3 customized keys (nodeSelector values added to empty maps)
	if result.Customized != 3 {
		t.Errorf("expected 3 customized keys, got %d", result.Customized)
		for _, e := range result.Entries {
			t.Logf("  %s: %s = %v", e.Classification, e.Path, e.UserValue)
		}
	}

	// Should have NO unknown keys - this is the critical test for the empty map fix
	if result.Unknown != 0 {
		t.Errorf("expected 0 unknown keys (node selectors should be recognized as customized), got %d", result.Unknown)
		for _, e := range result.Entries {
			if e.Classification == values.Unknown {
				t.Logf("  UNKNOWN: %s = %v", e.Path, e.UserValue)
			}
		}
	}
}

func TestScenario_Resources(t *testing.T) {
	if _, err := os.Stat(filepath.Join(testDataDir(), "defaults-v1.yaml")); os.IsNotExist(err) {
		t.Skip("test-data not available")
	}

	defaults := loadTestData(t, "defaults-v1.yaml")
	userValues := loadTestData(t, "scenario-resources.yaml")

	result := values.Classify(userValues, defaults)

	// Should have 8 customized keys (4 per resources block: cpu/memory for requests/limits)
	if result.Customized != 8 {
		t.Errorf("expected 8 customized keys, got %d", result.Customized)
		for _, e := range result.Entries {
			t.Logf("  %s: %s = %v", e.Classification, e.Path, e.UserValue)
		}
	}

	// Resources added to empty maps should NOT be unknown
	if result.Unknown != 0 {
		t.Errorf("expected 0 unknown keys, got %d", result.Unknown)
	}
}

func TestScenario_UnknownKeys(t *testing.T) {
	if _, err := os.Stat(filepath.Join(testDataDir(), "defaults-v1.yaml")); os.IsNotExist(err) {
		t.Skip("test-data not available")
	}

	defaults := loadTestData(t, "defaults-v1.yaml")
	userValues := loadTestData(t, "scenario-unknown-keys.yaml")

	result := values.Classify(userValues, defaults)

	// All keys should be unknown since they don't exist in defaults
	if result.Unknown != result.Total {
		t.Errorf("expected all %d keys to be unknown, got %d unknown", result.Total, result.Unknown)
	}

	// Should have zero customized or copied defaults
	if result.Customized != 0 {
		t.Errorf("expected 0 customized, got %d", result.Customized)
	}
	if result.CopiedDefault != 0 {
		t.Errorf("expected 0 copied defaults, got %d", result.CopiedDefault)
	}
}

func TestScenario_Mixed(t *testing.T) {
	if _, err := os.Stat(filepath.Join(testDataDir(), "defaults-v1.yaml")); os.IsNotExist(err) {
		t.Skip("test-data not available")
	}

	defaults := loadTestData(t, "defaults-v1.yaml")
	userValues := loadTestData(t, "scenario-mixed.yaml")

	result := values.Classify(userValues, defaults)

	// Should have customized keys (auth, architecture, image tag, nodeSelector, resources, replicaCount)
	if result.Customized < 5 {
		t.Errorf("expected at least 5 customized keys, got %d", result.Customized)
	}

	// Should have no unknown keys (all are valid chart keys)
	if result.Unknown != 0 {
		t.Errorf("expected 0 unknown keys, got %d", result.Unknown)
		for _, e := range result.Entries {
			if e.Classification == values.Unknown {
				t.Logf("  UNKNOWN: %s = %v", e.Path, e.UserValue)
			}
		}
	}

	t.Logf("Classification summary: customized=%d, copiedDefault=%d, unknown=%d",
		result.Customized, result.CopiedDefault, result.Unknown)
}

func TestScenario_Tolerations(t *testing.T) {
	if _, err := os.Stat(filepath.Join(testDataDir(), "defaults-v1.yaml")); os.IsNotExist(err) {
		t.Skip("test-data not available")
	}

	defaults := loadTestData(t, "defaults-v1.yaml")
	userValues := loadTestData(t, "scenario-tolerations.yaml")

	result := values.Classify(userValues, defaults)

	// Tolerations arrays should be detected as customized
	if result.Customized != 2 {
		t.Errorf("expected 2 customized keys (primary.tolerations, readReplicas.tolerations), got %d", result.Customized)
		for _, e := range result.Entries {
			t.Logf("  %s: %s = %v", e.Classification, e.Path, e.UserValue)
		}
	}
}

func TestUpgradeScenario_PreserveCustomizations(t *testing.T) {
	if _, err := os.Stat(filepath.Join(testDataDir(), "defaults-v1.yaml")); os.IsNotExist(err) {
		t.Skip("test-data not available")
	}

	oldDefaults := loadTestData(t, "defaults-v1.yaml")
	newDefaults := loadTestData(t, "defaults-v2.yaml")
	userValues := loadTestData(t, "scenario-auth-customized.yaml")

	// Merge user values
	upgraded := values.Merge(userValues, oldDefaults, newDefaults)

	// User's customized auth settings should be preserved
	if upgraded["global.postgresql.auth.existingSecret"] != "postgres-credentials" {
		t.Errorf("expected existingSecret to be preserved, got %v",
			upgraded["global.postgresql.auth.existingSecret"])
	}

	// New defaults should be applied for non-customized values
	// v1 (15.2.8) has image.tag: 16.2.0-debian-12-r18
	// v2 (18.2.0) has image.tag: latest
	if upgraded["image.tag"] != "latest" {
		t.Errorf("expected image.tag to be updated to new default 'latest', got %v",
			upgraded["image.tag"])
	}

	// New keys from v2 (18.2.0) should be present
	// global.security.allowInsecureImages is new in v2
	if upgraded["global.security.allowInsecureImages"] != false {
		t.Errorf("expected global.security.allowInsecureImages from v2 to be false, got %v",
			upgraded["global.security.allowInsecureImages"])
	}

	// global.defaultFips is also new in v2
	if upgraded["global.defaultFips"] != "restricted" {
		t.Errorf("expected global.defaultFips from v2 to be 'restricted', got %v",
			upgraded["global.defaultFips"])
	}
}

func TestUpgradeScenario_PreserveNodeSelectors(t *testing.T) {
	if _, err := os.Stat(filepath.Join(testDataDir(), "defaults-v1.yaml")); os.IsNotExist(err) {
		t.Skip("test-data not available")
	}

	oldDefaults := loadTestData(t, "defaults-v1.yaml")
	newDefaults := loadTestData(t, "defaults-v2.yaml")
	userValues := loadTestData(t, "scenario-node-selectors.yaml")

	upgraded := values.Merge(userValues, oldDefaults, newDefaults)

	// Node selector customizations should be preserved
	if upgraded["primary.nodeSelector.workload-type"] != "database" {
		t.Errorf("expected primary.nodeSelector.workload-type to be preserved, got %v",
			upgraded["primary.nodeSelector.workload-type"])
	}

	if upgraded["readReplicas.nodeSelector.workload-type"] != "database" {
		t.Errorf("expected readReplicas.nodeSelector.workload-type to be preserved, got %v",
			upgraded["readReplicas.nodeSelector.workload-type"])
	}
}
