package config

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

func TestConfigInitValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".solidify.yml")
	if err := os.WriteFile(path, InitYAML(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Validate(path); err != nil {
		t.Fatal(err)
	}
}

func TestConfigSchemaMatchesCheckedInArtifact(t *testing.T) {
	generated, err := SchemaJSON()
	if err != nil {
		t.Fatal(err)
	}
	_, filename, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(filename), "..", "..", "schemas", "solidlint-config-v1.schema.json")
	checkedIn, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, checkedIn) {
		t.Fatal("generated config schema differs from schemas/solidlint-config-v1.schema.json")
	}
}

func TestConfigValidateReportsLineAndCanonicalSuggestion(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".solidify.yml")
	if err := os.WriteFile(path, []byte("thresholds:\n  max_methodz: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Validate(path)
	if err == nil || !strings.Contains(err.Error(), ":2:") || !strings.Contains(err.Error(), "max_methods") {
		t.Fatalf("validation error = %v", err)
	}
}

func TestThresholdRegistryDrivesDefaultsAndBounds(t *testing.T) {
	defaults := analyzer.DefaultConfig()
	values := analyzer.EffectiveThresholds(defaults)
	for _, spec := range analyzer.ThresholdSpecs() {
		value, ok := values[spec.Name]
		if !ok {
			t.Fatalf("threshold %q missing from effective values", spec.Name)
		}
		if value < spec.Minimum || spec.Maximum != nil && value > *spec.Maximum {
			t.Fatalf("default %s=%d violates registry bounds", spec.Name, value)
		}
	}
}
