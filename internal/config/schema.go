package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
)

const SchemaVersion = 1

// InitYAML returns a canonical, immediately valid starter configuration.
func InitYAML() []byte {
	defaults := analyzer.DefaultConfig()
	values := analyzer.EffectiveThresholds(defaults)
	var output bytes.Buffer
	output.WriteString("profile: stable\n")
	output.WriteString("enabled_rules: []\n")
	output.WriteString("enabled_checks: []\n")
	output.WriteString("disabled_checks: []\n")
	output.WriteString("exclude: []\n")
	output.WriteString("thresholds:\n")
	for _, spec := range analyzer.ThresholdSpecs() {
		fmt.Fprintf(&output, "  %s: %d\n", spec.Name, values[spec.Name])
	}
	output.WriteString("severity: {}\n")
	output.WriteString("allow_dependencies: []\n")
	return output.Bytes()
}

// Validate checks syntax, known fields, semantic ranges, and suggestions.
func Validate(path string) error {
	_, err := Load(path)
	return err
}

// SchemaJSON returns the canonical JSON Schema for .solidify.yml.
func SchemaJSON() ([]byte, error) {
	thresholdProperties := map[string]any{}
	for _, spec := range analyzer.ThresholdSpecs() {
		property := map[string]any{
			"type": "integer", "minimum": spec.Minimum, "description": spec.Description,
		}
		if spec.Maximum != nil {
			property["maximum"] = *spec.Maximum
		}
		thresholdProperties[spec.Name] = property
	}
	checkIDs := make([]string, 0, len(analyzer.RegisteredCheckIDs()))
	for _, id := range analyzer.RegisteredCheckIDs() {
		checkIDs = append(checkIDs, string(id))
	}
	sort.Strings(checkIDs)
	properties := schemaProperties(checkIDs, thresholdProperties)
	document := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://github.com/ExtroNovosib/solidify/schemas/solidlint-config-v1.schema.json",
		"title":                "solidlint configuration",
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
	}
	data, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func schemaProperties(checkIDs []string, thresholds map[string]any) map[string]any {
	stringArray := func() map[string]any {
		return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "uniqueItems": true}
	}
	checkArray := func() map[string]any {
		return map[string]any{"type": "array", "items": map[string]any{"enum": checkIDs}, "uniqueItems": true}
	}
	severityValues := []string{"note", "warning", "error"}
	return map[string]any{
		"profile":            map[string]any{"enum": []string{"stable", "all", "calibration"}},
		"enabled_rules":      map[string]any{"type": "array", "items": map[string]any{"enum": []string{"S", "O", "L", "I", "D"}}, "uniqueItems": true},
		"enabled_checks":     checkArray(),
		"disabled_checks":    checkArray(),
		"exclude":            stringArray(),
		"thresholds":         map[string]any{"type": "object", "additionalProperties": false, "properties": thresholds},
		"severity":           map[string]any{"type": "object", "additionalProperties": map[string]any{"enum": severityValues}},
		"allow_dependencies": stringArray(),
		"fail_level":         map[string]any{"enum": severityValues},
		"ocp":                nestedStringArrays("discriminator_fields", "allow_dispatch_types", "allow_packages"),
		"architecture":       nestedStringArrays("logic_packages", "implementation_packages", "composition_roots"),
		"dip":                nestedStringArrays("infra_error_packages", "transport_types"),
	}
}

func nestedStringArrays(names ...string) map[string]any {
	properties := map[string]any{}
	for _, name := range names {
		properties[name] = map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "uniqueItems": true}
	}
	return map[string]any{"type": "object", "additionalProperties": false, "properties": properties}
}
