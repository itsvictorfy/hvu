package values

import (
	"testing"
)

func TestParseYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		check   func(t *testing.T, v Values)
	}{
		{
			name: "simple flat values",
			yaml: `
key1: value1
key2: 123
key3: true
`,
			wantErr: false,
			check: func(t *testing.T, v Values) {
				if v["key1"] != "value1" {
					t.Errorf("expected key1=value1, got %v", v["key1"])
				}
				if v["key2"] != 123 {
					t.Errorf("expected key2=123, got %v", v["key2"])
				}
				if v["key3"] != true {
					t.Errorf("expected key3=true, got %v", v["key3"])
				}
			},
		},
		{
			name: "nested values",
			yaml: `
parent:
  child1: value1
  child2:
    grandchild: value2
`,
			wantErr: false,
			check: func(t *testing.T, v Values) {
				if v["parent::child1"] != "value1" {
					t.Errorf("expected parent::child1=value1, got %v", v["parent::child1"])
				}
				if v["parent::child2::grandchild"] != "value2" {
					t.Errorf("expected parent::child2::grandchild=value2, got %v", v["parent::child2::grandchild"])
				}
			},
		},
		{
			name: "empty map preserved",
			yaml: `
parent:
  emptyMap: {}
  nonEmpty:
    key: value
`,
			wantErr: false,
			check: func(t *testing.T, v Values) {
				// Empty maps should be preserved
				if _, ok := v["parent::emptyMap"]; !ok {
					t.Error("expected parent::emptyMap to exist")
				}
				if v["parent::nonEmpty::key"] != "value" {
					t.Errorf("expected parent::nonEmpty::key=value, got %v", v["parent::nonEmpty::key"])
				}
			},
		},
		{
			name:    "invalid yaml",
			yaml:    `{invalid: yaml: content`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseYAML(tt.yaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil {
				tt.check(t, v)
			}
		})
	}
}

func TestClassify_ExactMatch(t *testing.T) {
	// When user values exactly match defaults, they should be classified as COPIED_DEFAULT
	defaults := Values{
		"key1":        "value1",
		"key2":        123,
		"nested::key": "nestedValue",
	}

	userValues := Values{
		"key1":        "value1",
		"key2":        123,
		"nested::key": "nestedValue",
	}

	result := Classify(userValues, defaults)

	if result.Customized != 0 {
		t.Errorf("expected 0 customized, got %d", result.Customized)
	}
	if result.CopiedDefault != 3 {
		t.Errorf("expected 3 copied defaults, got %d", result.CopiedDefault)
	}
	if result.Unknown != 0 {
		t.Errorf("expected 0 unknown, got %d", result.Unknown)
	}
}

func TestClassify_Customized(t *testing.T) {
	// When user values differ from defaults, they should be classified as CUSTOMIZED
	defaults := Values{
		"key1": "default1",
		"key2": "default2",
		"key3": "default3",
	}

	userValues := Values{
		"key1": "custom1",  // changed
		"key2": "default2", // same
		"key3": "custom3",  // changed
	}

	result := Classify(userValues, defaults)

	if result.Customized != 2 {
		t.Errorf("expected 2 customized, got %d", result.Customized)
	}
	if result.CopiedDefault != 1 {
		t.Errorf("expected 1 copied default, got %d", result.CopiedDefault)
	}
	if result.Unknown != 0 {
		t.Errorf("expected 0 unknown, got %d", result.Unknown)
	}

	// Verify specific classifications
	for _, entry := range result.Entries {
		switch entry.Path {
		case "key1", "key3":
			if entry.Classification != Customized {
				t.Errorf("expected %s to be CUSTOMIZED, got %s", entry.Path, entry.Classification)
			}
		case "key2":
			if entry.Classification != CopiedDefault {
				t.Errorf("expected %s to be COPIED_DEFAULT, got %s", entry.Path, entry.Classification)
			}
		}
	}
}

func TestClassify_Unknown(t *testing.T) {
	// When user values have keys not in defaults, they should be classified as UNKNOWN
	defaults := Values{
		"known1": "value1",
		"known2": "value2",
	}

	userValues := Values{
		"known1":  "value1",
		"unknown": "value3",
	}

	result := Classify(userValues, defaults)

	if result.Unknown != 1 {
		t.Errorf("expected 1 unknown, got %d", result.Unknown)
	}
	if result.CopiedDefault != 1 {
		t.Errorf("expected 1 copied default, got %d", result.CopiedDefault)
	}
}

func TestClassify_EmptyMapParent(t *testing.T) {
	// When defaults have an empty map and user adds content to it,
	// it should be classified as CUSTOMIZED (not UNKNOWN)
	defaults := Values{
		"nodeSelector":  map[string]interface{}{}, // empty map
		"resources":     map[string]interface{}{}, // empty map
		"existingValue": "default",
	}

	userValues := Values{
		"nodeSelector::workload-type": "database", // added to empty map
		"resources::requests::cpu":    "500m",     // added to empty map
		"resources::requests::memory": "512Mi",    // added to empty map
		"existingValue":               "default",  // matches default
	}

	result := Classify(userValues, defaults)

	if result.Customized != 3 {
		t.Errorf("expected 3 customized (values added to empty maps), got %d", result.Customized)
	}
	if result.Unknown != 0 {
		t.Errorf("expected 0 unknown (parent empty maps should count), got %d", result.Unknown)
	}
	if result.CopiedDefault != 1 {
		t.Errorf("expected 1 copied default, got %d", result.CopiedDefault)
	}
}

func TestClassify_NestedEmptyMap(t *testing.T) {
	// Test deeply nested empty maps
	defaults := Values{
		"primary::nodeSelector": map[string]interface{}{},
		"primary::resources":    map[string]interface{}{},
		"primary::tolerations":  []interface{}{},
	}

	userValues := Values{
		"primary::nodeSelector::tier":       "database",
		"primary::nodeSelector::env":        "prod",
		"primary::resources::requests::cpu": "1",
	}

	result := Classify(userValues, defaults)

	if result.Customized != 3 {
		t.Errorf("expected 3 customized, got %d", result.Customized)
	}
	if result.Unknown != 0 {
		t.Errorf("expected 0 unknown, got %d", result.Unknown)
	}
}

func TestMerge_BasicMerge(t *testing.T) {
	oldDefaults := Values{
		"key1": "old1",
		"key2": "old2",
		"key3": "old3",
	}

	newDefaults := Values{
		"key1": "new1",
		"key2": "new2",
		"key3": "new3",
		"key4": "new4", // new key in new version
	}

	userValues := Values{
		"key1": "custom1", // customized - should be kept
		"key2": "old2",    // matches old default - should be updated
		"key3": "old3",    // matches old default - should be updated
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	// Customized value should be preserved
	if result["key1"] != "custom1" {
		t.Errorf("expected key1=custom1 (preserved), got %v", result["key1"])
	}

	// Values matching old defaults should be updated to new defaults
	if result["key2"] != "new2" {
		t.Errorf("expected key2=new2 (updated), got %v", result["key2"])
	}
	if result["key3"] != "new3" {
		t.Errorf("expected key3=new3 (updated), got %v", result["key3"])
	}

	// New key should be present
	if result["key4"] != "new4" {
		t.Errorf("expected key4=new4 (new), got %v", result["key4"])
	}
}

func TestMerge_PreservesUnknownKeys(t *testing.T) {
	oldDefaults := Values{
		"known": "old",
	}

	newDefaults := Values{
		"known": "new",
	}

	userValues := Values{
		"known":   "customized", // customized
		"unknown": "userValue",  // not in any defaults
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	// Unknown keys should be preserved (they might be custom additions)
	if result["unknown"] != "userValue" {
		t.Errorf("expected unknown=userValue (preserved), got %v", result["unknown"])
	}

	// Customized known key should be preserved
	if result["known"] != "customized" {
		t.Errorf("expected known=customized (preserved), got %v", result["known"])
	}
}

func TestToYAML(t *testing.T) {
	v := Values{
		"simple":            "value",
		"nested::child":     "childValue",
		"deep::nested::key": 123,
	}

	yaml, err := v.ToYAML()
	if err != nil {
		t.Fatalf("ToYAML() error = %v", err)
	}

	// Parse it back to verify structure
	parsed, err := ParseYAML(yaml)
	if err != nil {
		t.Fatalf("failed to parse generated YAML: %v", err)
	}

	if parsed["simple"] != "value" {
		t.Errorf("expected simple=value, got %v", parsed["simple"])
	}
	if parsed["nested::child"] != "childValue" {
		t.Errorf("expected nested.child=childValue, got %v", parsed["nested::child"])
	}
	if parsed["deep::nested::key"] != 123 {
		t.Errorf("expected deep.nested.key=123, got %v", parsed["deep::nested::key"])
	}
}

func TestGetPaths(t *testing.T) {
	v := Values{
		"b": 1,
		"a": 2,
		"c": 3,
	}

	paths := v.GetPaths()

	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}

	// Should be sorted
	if paths[0] != "a" || paths[1] != "b" || paths[2] != "c" {
		t.Errorf("expected sorted paths [a, b, c], got %v", paths)
	}
}

func TestClassify_EntriesSorted(t *testing.T) {
	defaults := Values{
		"z": "1",
		"a": "2",
		"m": "3",
	}

	userValues := Values{
		"z": "1",
		"a": "2",
		"m": "3",
	}

	result := Classify(userValues, defaults)

	// Entries should be sorted by path
	if len(result.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result.Entries))
	}
	if result.Entries[0].Path != "a" {
		t.Errorf("expected first entry to be 'a', got '%s'", result.Entries[0].Path)
	}
	if result.Entries[1].Path != "m" {
		t.Errorf("expected second entry to be 'm', got '%s'", result.Entries[1].Path)
	}
	if result.Entries[2].Path != "z" {
		t.Errorf("expected third entry to be 'z', got '%s'", result.Entries[2].Path)
	}
}

func TestClassify_ArrayValues(t *testing.T) {
	defaults := Values{
		"emptyArray":  []interface{}{},
		"filledArray": []interface{}{"a", "b"},
	}

	userValues := Values{
		"emptyArray":  []interface{}{"custom"}, // changed from empty
		"filledArray": []interface{}{"a", "b"}, // same as default
	}

	result := Classify(userValues, defaults)

	if result.Customized != 1 {
		t.Errorf("expected 1 customized, got %d", result.Customized)
	}
	if result.CopiedDefault != 1 {
		t.Errorf("expected 1 copied default, got %d", result.CopiedDefault)
	}
}

// =============================================================================
// Upgrade/Merge Tests
// =============================================================================

func TestMerge_CopiedDefaultsGetUpdated(t *testing.T) {
	// Values that match old defaults should be updated to new defaults
	oldDefaults := Values{
		"image::tag":        "15.0.0",
		"image::repository": "bitnami/postgresql",
		"replicaCount":      1,
	}

	newDefaults := Values{
		"image::tag":        "16.0.0", // updated
		"image::repository": "bitnami/postgresql",
		"replicaCount":      2, // updated
	}

	userValues := Values{
		"image::tag":        "15.0.0", // matches old default - should update
		"image::repository": "bitnami/postgresql",
		"replicaCount":      1, // matches old default - should update
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	if result["image::tag"] != "16.0.0" {
		t.Errorf("expected image.tag to be updated to 16.0.0, got %v", result["image::tag"])
	}
	if result["replicaCount"] != 2 {
		t.Errorf("expected replicaCount to be updated to 2, got %v", result["replicaCount"])
	}
}

func TestMerge_CustomizedValuesPreserved(t *testing.T) {
	// User customizations should be preserved even when new defaults change
	oldDefaults := Values{
		"image::tag":   "15.0.0",
		"replicaCount": 1,
		"memory":       "512Mi",
	}

	newDefaults := Values{
		"image::tag":   "16.0.0",
		"replicaCount": 2,
		"memory":       "1Gi",
	}

	userValues := Values{
		"image::tag":   "15.5.0", // customized - should preserve
		"replicaCount": 5,        // customized - should preserve
		"memory":       "2Gi",    // customized - should preserve
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	if result["image::tag"] != "15.5.0" {
		t.Errorf("expected customized image.tag to be preserved as 15.5.0, got %v", result["image::tag"])
	}
	if result["replicaCount"] != 5 {
		t.Errorf("expected customized replicaCount to be preserved as 5, got %v", result["replicaCount"])
	}
	if result["memory"] != "2Gi" {
		t.Errorf("expected customized memory to be preserved as 2Gi, got %v", result["memory"])
	}
}

func TestMerge_NewKeysAdded(t *testing.T) {
	// New keys in new defaults should be added to result
	oldDefaults := Values{
		"existingKey": "value",
	}

	newDefaults := Values{
		"existingKey":    "value",
		"newFeature":     true,
		"newConfig.host": "localhost",
		"newConfig.port": 8080,
	}

	userValues := Values{
		"existingKey": "value",
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	if result["newFeature"] != true {
		t.Errorf("expected newFeature to be added, got %v", result["newFeature"])
	}
	if result["newConfig.host"] != "localhost" {
		t.Errorf("expected newConfig.host to be added, got %v", result["newConfig.host"])
	}
	if result["newConfig.port"] != 8080 {
		t.Errorf("expected newConfig.port to be added, got %v", result["newConfig.port"])
	}
}

func TestMerge_RemovedKeysFromDefaults(t *testing.T) {
	// Keys removed from new defaults but customized by user should be preserved
	oldDefaults := Values{
		"deprecatedKey": "oldValue",
		"keptKey":       "value",
	}

	newDefaults := Values{
		"keptKey": "newValue",
		// deprecatedKey removed in new version
	}

	userValues := Values{
		"deprecatedKey": "customValue", // customized - should preserve
		"keptKey":       "value",       // matches old default - should update
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	// Customized deprecated key should be preserved
	if result["deprecatedKey"] != "customValue" {
		t.Errorf("expected customized deprecatedKey to be preserved, got %v", result["deprecatedKey"])
	}
	// Key that matched old default should update to new default
	if result["keptKey"] != "newValue" {
		t.Errorf("expected keptKey to be updated to newValue, got %v", result["keptKey"])
	}
}

func TestMerge_UnknownUserKeysPreserved(t *testing.T) {
	// User keys not in any defaults should be preserved
	oldDefaults := Values{
		"known": "old",
	}

	newDefaults := Values{
		"known": "new",
	}

	userValues := Values{
		"known":           "old",        // matches old default - should update
		"customAddition":  "userValue",  // not in defaults - should preserve
		"another::custom": "anotherVal", // not in defaults - should preserve
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	if result["customAddition"] != "userValue" {
		t.Errorf("expected customAddition to be preserved, got %v", result["customAddition"])
	}
	if result["another::custom"] != "anotherVal" {
		t.Errorf("expected another.custom to be preserved, got %v", result["another::custom"])
	}
}

func TestMerge_EmptyMapHandling(t *testing.T) {
	// User values added to empty maps should be preserved
	oldDefaults := Values{
		"nodeSelector": map[string]interface{}{},
		"resources":    map[string]interface{}{},
	}

	newDefaults := Values{
		"nodeSelector": map[string]interface{}{},
		"resources":    map[string]interface{}{},
		"newEmpty":     map[string]interface{}{},
	}

	userValues := Values{
		"nodeSelector::tier":        "database",
		"resources::requests::cpu":  "500m",
		"resources::limits::memory": "1Gi",
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	// User additions to empty maps should be preserved
	if result["nodeSelector::tier"] != "database" {
		t.Errorf("expected nodeSelector.tier to be preserved, got %v", result["nodeSelector::tier"])
	}
	if result["resources::requests::cpu"] != "500m" {
		t.Errorf("expected resources.requests.cpu to be preserved, got %v", result["resources::requests::cpu"])
	}
	if result["resources::limits::memory"] != "1Gi" {
		t.Errorf("expected resources.limits.memory to be preserved, got %v", result["resources::limits::memory"])
	}
}

func TestMerge_ArrayValuesHandling(t *testing.T) {
	oldDefaults := Values{
		"tolerations": []interface{}{},
		"env":         []interface{}{map[string]interface{}{"name": "FOO", "value": "bar"}},
	}

	newDefaults := Values{
		"tolerations": []interface{}{},
		"env":         []interface{}{map[string]interface{}{"name": "FOO", "value": "newbar"}},
	}

	userValues := Values{
		"tolerations": []interface{}{
			map[string]interface{}{"key": "node-role", "operator": "Exists"},
		}, // customized
		"env": []interface{}{map[string]interface{}{"name": "FOO", "value": "bar"}}, // matches old default
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	// Customized tolerations should be preserved
	tols, ok := result["tolerations"].([]interface{})
	if !ok || len(tols) != 1 {
		t.Errorf("expected tolerations to be preserved with 1 item, got %v", result["tolerations"])
	}

	// env that matched old default should be updated
	envs, ok := result["env"].([]interface{})
	if !ok || len(envs) != 1 {
		t.Errorf("expected env to be updated, got %v", result["env"])
	}
}

func TestMerge_TypeChanges(t *testing.T) {
	// Handle cases where value type changes between versions
	oldDefaults := Values{
		"config": "simple-string",
	}

	newDefaults := Values{
		"config": map[string]interface{}{"enabled": true}, // type changed
	}

	userValues := Values{
		"config": "simple-string", // matches old default
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	// Should update to new default (type changed)
	configMap, ok := result["config"].(map[string]interface{})
	if !ok {
		t.Errorf("expected config to be updated to map type, got %T: %v", result["config"], result["config"])
	} else if configMap["enabled"] != true {
		t.Errorf("expected config.enabled to be true, got %v", configMap["enabled"])
	}
}

func TestMerge_BooleanValues(t *testing.T) {
	oldDefaults := Values{
		"feature::enabled":  false,
		"feature::disabled": true,
	}

	newDefaults := Values{
		"feature::enabled":  true,  // changed default
		"feature::disabled": false, // changed default
	}

	userValues := Values{
		"feature::enabled":  true, // customized (differs from old default false)
		"feature::disabled": true, // matches old default
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	// Customized to true should be preserved
	if result["feature::enabled"] != true {
		t.Errorf("expected feature.enabled to be preserved as true, got %v", result["feature::enabled"])
	}
	// Matched old default true, should update to new default false
	if result["feature::disabled"] != false {
		t.Errorf("expected feature.disabled to be updated to false, got %v", result["feature::disabled"])
	}
}

func TestMerge_NumericValues(t *testing.T) {
	oldDefaults := Values{
		"replicas":   1,
		"port":       8080,
		"timeout":    30.5,
		"maxRetries": 3,
	}

	newDefaults := Values{
		"replicas":   2,
		"port":       9090,
		"timeout":    60.0,
		"maxRetries": 5,
	}

	userValues := Values{
		"replicas":   1,    // matches old default - update
		"port":       3000, // customized - preserve
		"timeout":    30.5, // matches old default - update
		"maxRetries": 10,   // customized - preserve
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	if result["replicas"] != 2 {
		t.Errorf("expected replicas to be updated to 2, got %v", result["replicas"])
	}
	if result["port"] != 3000 {
		t.Errorf("expected port to be preserved as 3000, got %v", result["port"])
	}
	if result["timeout"] != 60.0 {
		t.Errorf("expected timeout to be updated to 60.0, got %v", result["timeout"])
	}
	if result["maxRetries"] != 10 {
		t.Errorf("expected maxRetries to be preserved as 10, got %v", result["maxRetries"])
	}
}

func TestMerge_MixedScenario(t *testing.T) {
	// Comprehensive test with mixed scenarios
	oldDefaults := Values{
		"image::tag":            "15.0.0",
		"image::repository":     "bitnami/postgresql",
		"primary::replicaCount": 1,
		"primary::nodeSelector": map[string]interface{}{},
		"primary::resources":    map[string]interface{}{},
		"auth::enabled":         true,
		"auth::database":        "postgres",
		"metrics::enabled":      false,
		"deprecatedFeature":     "old",
	}

	newDefaults := Values{
		"image::tag":            "16.0.0",
		"image::repository":     "bitnami/postgresql",
		"primary::replicaCount": 2,
		"primary::nodeSelector": map[string]interface{}{},
		"primary::resources":    map[string]interface{}{},
		"auth::enabled":         true,
		"auth::database":        "app", // default changed
		"metrics::enabled":      true,  // default changed
		"newFeature::enabled":   false, // new key
		// deprecatedFeature removed
	}

	userValues := Values{
		"image::tag":                           "15.5.0",             // customized
		"image::repository":                    "bitnami/postgresql", // matches default
		"primary::replicaCount":                1,                    // matches old default
		"primary::nodeSelector::workload-type": "database",           // added to empty map
		"primary::resources::requests::cpu":    "500m",               // added to empty map
		"auth::enabled":                        true,                 // matches both defaults
		"auth::database":                       "mydb",               // customized
		"metrics::enabled":                     false,                // matches old default
		"deprecatedFeature":                    "custom",             // customized deprecated key
		"customKey":                            "userValue",          // unknown key
	}

	result := Merge(userValues, oldDefaults, newDefaults)

	// Customized values preserved
	if result["image::tag"] != "15.5.0" {
		t.Errorf("image.tag: expected 15.5.0, got %v", result["image::tag"])
	}
	if result["auth::database"] != "mydb" {
		t.Errorf("auth.database: expected mydb, got %v", result["auth::database"])
	}
	if result["deprecatedFeature"] != "custom" {
		t.Errorf("deprecatedFeature: expected custom, got %v", result["deprecatedFeature"])
	}

	// Values matching old defaults updated
	if result["primary::replicaCount"] != 2 {
		t.Errorf("primary.replicaCount: expected 2, got %v", result["primary::replicaCount"])
	}
	if result["metrics::enabled"] != true {
		t.Errorf("metrics.enabled: expected true, got %v", result["metrics::enabled"])
	}

	// Empty map additions preserved
	if result["primary::nodeSelector::workload-type"] != "database" {
		t.Errorf("primary.nodeSelector.workload-type: expected database, got %v", result["primary::nodeSelector::workload-type"])
	}
	if result["primary::resources::requests::cpu"] != "500m" {
		t.Errorf("primary.resources.requests.cpu: expected 500m, got %v", result["primary::resources::requests::cpu"])
	}

	// New keys added
	if result["newFeature::enabled"] != false {
		t.Errorf("newFeature.enabled: expected false, got %v", result["newFeature::enabled"])
	}

	// Unknown keys preserved
	if result["customKey"] != "userValue" {
		t.Errorf("customKey: expected userValue, got %v", result["customKey"])
	}
}

func TestUnflatten_EmptyMapDoesNotOverwriteChildren(t *testing.T) {
	// Regression test: Empty map parents should not overwrite child keys during unflatten
	// This was a bug where if both "pdb: {}" and "pdb.create: true" existed in the flat map,
	// the iteration order could cause "pdb: {}" to overwrite the map containing "pdb::create"
	flat := Values{
		"pdb":                       map[string]interface{}{}, // empty map parent
		"pdb::create":               true,                     // child key
		"pdb::minAvailable":         0,                        // another child key
		"resources.limits":          map[string]interface{}{}, // empty map parent
		"resources::limits::cpu":    "500m",                   // child key
		"resources::limits::memory": "5Gi",                    // another child key
	}

	nested := Unflatten(flat)

	// Convert back to Values for easier verification
	result := Flatten(nested)

	// Verify all child keys are preserved
	if result["pdb::create"] != true {
		t.Errorf("expected pdb.create=true, got %v", result["pdb::create"])
	}
	if result["pdb::minAvailable"] != 0 {
		t.Errorf("expected pdb.minAvailable=0, got %v", result["pdb::minAvailable"])
	}
	if result["resources::limits::cpu"] != "500m" {
		t.Errorf("expected resources.limits.cpu=500m, got %v", result["resources::limits::cpu"])
	}
	if result["resources::limits::memory"] != "5Gi" {
		t.Errorf("expected resources.limits.memory=5Gi, got %v", result["resources::limits::memory"])
	}

	// Verify the parents are maps (not empty)
	pdb, ok := nested["pdb"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected pdb to be a map, got %T", nested["pdb"])
	}
	if len(pdb) != 2 {
		t.Errorf("expected pdb to have 2 children, got %d: %v", len(pdb), pdb)
	}

	resources, ok := nested["resources"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected resources to be a map, got %T", nested["resources"])
	}
	limits, ok := resources["limits"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected resources.limits to be a map, got %T", resources["limits"])
	}
	if len(limits) != 2 {
		t.Errorf("expected resources.limits to have 2 children, got %d: %v", len(limits), limits)
	}
}

func TestMerge_EmptyInputs(t *testing.T) {
	t.Run("empty user values", func(t *testing.T) {
		oldDefaults := Values{"key": "old"}
		newDefaults := Values{"key": "new", "newKey": "value"}
		userValues := Values{}

		result := Merge(userValues, oldDefaults, newDefaults)

		if result["key"] != "new" {
			t.Errorf("expected key=new, got %v", result["key"])
		}
		if result["newKey"] != "value" {
			t.Errorf("expected newKey=value, got %v", result["newKey"])
		}
	})

	t.Run("empty old defaults", func(t *testing.T) {
		oldDefaults := Values{}
		newDefaults := Values{"key": "new"}
		userValues := Values{"key": "custom", "userKey": "userVal"}

		result := Merge(userValues, oldDefaults, newDefaults)

		// User values are all "customized" since nothing matched old defaults
		if result["key"] != "custom" {
			t.Errorf("expected key=custom, got %v", result["key"])
		}
		if result["userKey"] != "userVal" {
			t.Errorf("expected userKey=userVal, got %v", result["userKey"])
		}
	})

	t.Run("empty new defaults", func(t *testing.T) {
		oldDefaults := Values{"key": "old"}
		newDefaults := Values{}
		userValues := Values{"key": "custom"}

		result := Merge(userValues, oldDefaults, newDefaults)

		// Customized value should be preserved
		if result["key"] != "custom" {
			t.Errorf("expected key=custom, got %v", result["key"])
		}
	})
}
