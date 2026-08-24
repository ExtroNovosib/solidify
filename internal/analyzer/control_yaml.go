package analyzer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type yamlFileConfig struct {
	Profile           string            `yaml:"profile"`
	EnabledRules      []string          `yaml:"enabled_rules"`
	EnabledChecks     []string          `yaml:"enabled_checks"`
	Exclude           []string          `yaml:"exclude"`
	Thresholds        map[string]int    `yaml:"thresholds"`
	Severity          map[string]string `yaml:"severity"`
	AllowDependencies []string          `yaml:"allow_dependencies"`
	DisabledChecks    []string          `yaml:"disabled_checks"`
	FailLevel         string            `yaml:"fail_level"`
	OCP               yamlOCPSection    `yaml:"ocp"`
	Architecture      yamlArchSection   `yaml:"architecture"`
	DIP               yamlDIPSection    `yaml:"dip"`
}

type yamlOCPSection struct {
	DiscriminatorFields []string `yaml:"discriminator_fields"`
	AllowDispatchTypes  []string `yaml:"allow_dispatch_types"`
	AllowPackages       []string `yaml:"allow_packages"`
}

type yamlArchSection struct {
	LogicPackages          []string `yaml:"logic_packages"`
	ImplementationPackages []string `yaml:"implementation_packages"`
	CompositionRoots       []string `yaml:"composition_roots"`
}

type yamlDIPSection struct {
	InfraErrorPackages []string `yaml:"infra_error_packages"`
	TransportTypes     []string `yaml:"transport_types"`
}

func LoadFileConfig(path string) (FileConfig, error) {
	cfg := FileConfig{Thresholds: map[string]int{}, Severities: map[string]Severity{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	var raw yamlFileConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil {
		return cfg, yamlConfigError(path, data, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return cfg, fmt.Errorf("%s: multiple YAML documents are not supported", path)
		}
		return cfg, yamlConfigError(path, data, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	if err := raw.validate(path, &doc); err != nil {
		return cfg, err
	}
	return raw.toFileConfig(), nil
}

func (raw yamlFileConfig) validate(path string, doc *yaml.Node) error {
	return raw.validateSemantic(path, doc)
}

func (raw yamlFileConfig) validateSemantic(path string, doc *yaml.Node) error {
	if raw.Profile != "" && Profile(raw.Profile) != ProfileStable && Profile(raw.Profile) != ProfileAll && Profile(raw.Profile) != ProfileCalibration {
		return semanticConfigError(path, doc, []string{"profile"}, "profile must be stable, all, or calibration", "")
	}
	for _, rule := range raw.EnabledRules {
		if !validRuleCode(rule) {
			return semanticConfigError(path, doc, []string{"enabled_rules"}, fmt.Sprintf("unknown enabled rule %q", rule), nearestKnownRuleCode(rule))
		}
	}
	if err := ValidateExcludePatterns(raw.Exclude); err != nil {
		return semanticConfigError(path, doc, []string{"exclude"}, err.Error(), "")
	}
	enabled := map[string]bool{}
	for _, id := range raw.EnabledChecks {
		if !IsKnownCheckID(id) {
			return semanticConfigError(path, doc, []string{"enabled_checks"}, fmt.Sprintf("unknown enabled check %q", id), nearestKnownCheckID(id))
		}
		enabled[id] = true
	}
	for _, id := range raw.DisabledChecks {
		if !IsKnownCheckID(id) {
			return semanticConfigError(path, doc, []string{"disabled_checks"}, fmt.Sprintf("unknown disabled check %q", id), nearestKnownCheckID(id))
		}
		if enabled[id] {
			return semanticConfigError(path, doc, []string{"disabled_checks"}, fmt.Sprintf("check %q cannot be both enabled and disabled", id), "")
		}
	}
	fail := strings.TrimSpace(raw.FailLevel)
	if fail != "" && !validSeverity(Severity(fail)) {
		return semanticConfigError(path, doc, []string{"fail_level"}, "fail_level must be note, warning, or error", "")
	}
	for key, value := range raw.Severity {
		if !IsKnownSeverityTarget(key) {
			return semanticConfigError(path, doc, []string{"severity", key}, fmt.Sprintf("unknown severity target %q", key), nearestKnownSeverityTarget(key))
		}
		if !validSeverity(Severity(value)) {
			return semanticConfigError(path, doc, []string{"severity", key}, fmt.Sprintf("severity for %q must be note, warning, or error", key), "")
		}
	}
	for key := range raw.Thresholds {
		if !validThreshold(key) {
			return semanticConfigError(path, doc, []string{"thresholds", key}, fmt.Sprintf("unknown threshold %q", key), nearestKnownThreshold(key))
		}
		if raw.Thresholds[key] < 0 {
			return semanticConfigError(path, doc, []string{"thresholds", key}, fmt.Sprintf("threshold %q must be a non-negative integer", key), "")
		}
		if err := validateThresholdValue(key, raw.Thresholds[key]); err != nil {
			return semanticConfigError(path, doc, []string{"thresholds", key}, err.Error(), "")
		}
	}
	return nil
}

func (raw yamlFileConfig) toFileConfig() FileConfig {
	cfg := FileConfig{
		Profile:                   Profile(raw.Profile),
		EnabledRules:              append([]string(nil), raw.EnabledRules...),
		EnabledChecks:             append([]string(nil), raw.EnabledChecks...),
		Excludes:                  append([]string(nil), raw.Exclude...),
		Thresholds:                map[string]int{},
		Severities:                map[string]Severity{},
		AllowDependencies:         append([]string(nil), raw.AllowDependencies...),
		DisabledChecks:            append([]string(nil), raw.DisabledChecks...),
		FailLevel:                 Severity(strings.TrimSpace(raw.FailLevel)),
		OCPDiscriminatorFields:    append([]string(nil), raw.OCP.DiscriminatorFields...),
		OCPAllowDispatchTypes:     append([]string(nil), raw.OCP.AllowDispatchTypes...),
		OCPAllowPackages:          append([]string(nil), raw.OCP.AllowPackages...),
		OCPLogicPackages:          append([]string(nil), raw.Architecture.LogicPackages...),
		OCPImplementationPackages: append([]string(nil), raw.Architecture.ImplementationPackages...),
		OCPCompositionRoots:       append([]string(nil), raw.Architecture.CompositionRoots...),
		DIPInfraErrorPackages:     append([]string(nil), raw.DIP.InfraErrorPackages...),
		DIPTransportTypes:         append([]string(nil), raw.DIP.TransportTypes...),
	}
	for key, value := range raw.Thresholds {
		cfg.Thresholds[key] = value
	}
	for key, value := range raw.Severity {
		cfg.Severities[key] = Severity(value)
	}
	return cfg
}

func yamlConfigError(path string, data []byte, err error) error {
	message := err.Error()
	legacy := map[string]string{
		"enabled-rules": "enabled_rules", "enabled-checks": "enabled_checks",
		"disabled-checks": "disabled_checks", "allow-dependencies": "allow_dependencies",
		"fail-level": "fail_level", "severities": "severity", "excludes": "exclude",
		"discriminator-fields": "discriminator_fields", "allow-dispatch-types": "allow_dispatch_types",
		"allow-packages": "allow_packages", "logic-packages": "logic_packages",
		"implementation-packages": "implementation_packages", "composition-roots": "composition_roots",
		"infra-error-packages": "infra_error_packages", "transport-types": "transport_types",
	}
	for old, canonical := range legacy {
		if strings.Contains(message, "field "+old+" not found") {
			err = fmt.Errorf("unknown legacy key %q; use canonical key %q", old, canonical)
			break
		}
	}
	var node *yaml.Node
	if unmarshalErr := yaml.Unmarshal(data, &node); unmarshalErr == nil && node != nil {
		if line := yamlErrorLine(node, err); line > 0 {
			return fmt.Errorf("%s:%d: %w", path, line, err)
		}
	}
	return fmt.Errorf("%s: %w", path, err)
}

func yamlErrorLine(node *yaml.Node, err error) int {
	if node == nil {
		return 0
	}
	errorText := err.Error()
	if marker := strings.Index(errorText, "field "); marker >= 0 {
		fieldText := errorText[marker+len("field "):]
		field := strings.Fields(fieldText)
		if len(field) > 0 {
			var find func(*yaml.Node) int
			find = func(current *yaml.Node) int {
				if current == nil {
					return 0
				}
				if current.Kind == yaml.MappingNode {
					for index := 0; index+1 < len(current.Content); index += 2 {
						key, value := current.Content[index], current.Content[index+1]
						if key.Value == field[0] {
							return key.Line
						}
						if line := find(value); line > 0 {
							return line
						}
					}
				}
				for _, child := range current.Content {
					if line := find(child); line > 0 {
						return line
					}
				}
				return 0
			}
			if line := find(node); line > 0 {
				return line
			}
		}
	}
	if node.Line > 0 {
		return node.Line
	}
	return 0
}

func semanticConfigError(path string, doc *yaml.Node, fieldPath []string, message, suggestion string) error {
	if suggestion != "" {
		message = fmt.Sprintf("%s (did you mean %q?)", message, suggestion)
	}
	if line := yamlFieldLine(doc, fieldPath); line > 0 {
		return fmt.Errorf("%s:%d: %s", path, line, message)
	}
	return fmt.Errorf("%s: %s", path, message)
}

func yamlFieldLine(doc *yaml.Node, fieldPath []string) int {
	if doc == nil || len(fieldPath) == 0 {
		return 0
	}
	current := doc
	if current.Kind == yaml.DocumentNode && len(current.Content) > 0 {
		current = current.Content[0]
	}
	for index, segment := range fieldPath {
		if segment == "" {
			continue
		}
		if index == len(fieldPath)-1 {
			if line := yamlMapKeyLine(current, segment); line > 0 {
				return line
			}
			return yamlMapValueLine(current, segment)
		}
		current = yamlMapValueNode(current, segment)
		if current == nil {
			return 0
		}
	}
	return 0
}

func yamlMapKeyLine(node *yaml.Node, key string) int {
	if node == nil || node.Kind != yaml.MappingNode {
		return 0
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index].Line
		}
	}
	return 0
}

func yamlMapValueLine(node *yaml.Node, key string) int {
	if node == nil || node.Kind != yaml.MappingNode {
		return 0
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1].Line
		}
	}
	return 0
}

func yamlMapValueNode(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}
	return nil
}

func nearestKnownThreshold(input string) string {
	return nearestKnownField(input, thresholdNames())
}

func nearestKnownCheckID(input string) string {
	known := make([]string, 0, len(RegisteredCheckIDs()))
	for _, id := range RegisteredCheckIDs() {
		known = append(known, string(id))
	}
	return nearestKnownField(input, known)
}

func nearestKnownSeverityTarget(input string) string {
	known := []string{
		string(RuleSRP), string(RuleOCP), string(RuleLSP), string(RuleISP), string(RuleDIP),
	}
	for _, id := range RegisteredCheckIDs() {
		known = append(known, string(id))
	}
	return nearestKnownField(input, known)
}

func nearestKnownRuleCode(input string) string {
	return nearestKnownField(strings.ToUpper(strings.TrimSpace(input)), []string{"S", "O", "L", "I", "D"})
}

func nearestKnownField(input string, known []string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(input), "-", "_"))
	if normalized == "" {
		return ""
	}
	best := ""
	bestDistance := 3
	for _, candidate := range known {
		candidateNorm := strings.ToLower(strings.ReplaceAll(candidate, "-", "_"))
		distance := editDistance(normalized, candidateNorm)
		if distance < bestDistance {
			bestDistance = distance
			best = candidate
		}
	}
	return best
}

func editDistance(left, right string) int {
	if left == right {
		return 0
	}
	if len(left) == 0 {
		return len(right)
	}
	if len(right) == 0 {
		return len(left)
	}
	previous := make([]int, len(right)+1)
	current := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for i := 1; i <= len(left); i++ {
		current[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 1
			if left[i-1] == right[j-1] {
				cost = 0
			}
			current[j] = min(min(current[j-1]+1, previous[j]+1), previous[j-1]+cost)
		}
		copy(previous, current)
	}
	return previous[len(right)]
}
