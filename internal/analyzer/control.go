package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	thresholdISPUsage      = "isp_usage_ratio_percent"
	thresholdMinTCC        = "min_tcc_percent"
	thresholdOCPOverlap    = "ocp_dispatch_overlap_percent"
	thresholdOCPSimilarity = "ocp_parallel_similarity_percent"
)

// FileConfig is the intentionally small .solidify.yml configuration surface.
type FileConfig struct {
	Profile                   Profile
	EnabledRules              []string
	EnabledChecks             []string
	Excludes                  []string
	Thresholds                map[string]int
	Severities                map[string]Severity
	AllowDependencies         []string
	DisabledChecks            []string
	FailLevel                 Severity
	OCPDiscriminatorFields    []string
	OCPAllowDispatchTypes     []string
	OCPAllowPackages          []string
	OCPLogicPackages          []string
	OCPImplementationPackages []string
	OCPCompositionRoots       []string
	DIPInfraErrorPackages     []string
	DIPTransportTypes         []string
}

func FindConfig(start string) string {
	start = configSearchStart(start)
	absolute, err := filepath.Abs(start)
	if err == nil {
		start = absolute
	}
	for dir := start; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, ".solidify.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
	}
}

// FindConfigForTargets discovers configuration relative to the paths being
// scanned. A single run cannot safely combine different project policies, so
// mixed configured/unconfigured roots or multiple config files require an
// explicit -config selection.
func FindConfigForTargets(targets []string) (string, error) {
	configs := map[string]bool{}
	var withoutConfig []string
	for _, target := range targets {
		configPath := FindConfig(target)
		if configPath == "" {
			withoutConfig = append(withoutConfig, target)
			continue
		}
		configs[configPath] = true
	}
	if len(configs) == 0 {
		return "", nil
	}
	if len(configs) == 1 && len(withoutConfig) == 0 {
		for configPath := range configs {
			return configPath, nil
		}
	}
	found := make([]string, 0, len(configs))
	for configPath := range configs {
		found = append(found, configPath)
	}
	sort.Strings(found)
	sort.Strings(withoutConfig)
	return "", fmt.Errorf(
		"targets resolve to different configuration scopes (configs=%v, unconfigured=%v); pass -config explicitly or scan each project separately",
		found, withoutConfig,
	)
}

func configSearchStart(target string) string {
	if strings.TrimSpace(target) == "" {
		return "."
	}
	start := filepath.Clean(target)
	if filepath.Base(start) == "..." {
		start = filepath.Dir(start)
	}
	if info, err := os.Stat(start); err == nil && !info.IsDir() {
		start = filepath.Dir(start)
	}
	return start
}

func validThreshold(key string) bool {
	_, ok := thresholdSpec(key)
	return ok
}

func validateThresholdValue(key string, value int) error {
	spec, ok := thresholdSpec(key)
	if !ok {
		return fmt.Errorf("unknown threshold %q", key)
	}
	if spec.Maximum != nil && (value < spec.Minimum || value > *spec.Maximum) {
		return fmt.Errorf("threshold %q must be between %d and %d", key, spec.Minimum, *spec.Maximum)
	}
	if value < spec.Minimum {
		return fmt.Errorf("threshold %q must be at least %d", key, spec.Minimum)
	}
	return nil
}

func validRuleCode(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "S", "O", "L", "I", "D":
		return true
	default:
		return false
	}
}

// ValidateConfig applies semantic validation shared by YAML and CLI values.
// Keeping this separate from parsing ensures a negative or impossible CLI
// threshold cannot bypass the stricter file configuration path.
func ValidateConfig(cfg Config) error {
	for _, spec := range thresholdRegistry {
		if err := validateThresholdValue(spec.Name, spec.get(cfg)); err != nil {
			return err
		}
	}
	if cfg.AnalysisMode != "" && cfg.AnalysisMode != analysisModeAuto && cfg.AnalysisMode != syntaxAnalysisMode && cfg.AnalysisMode != "types" {
		return fmt.Errorf("unknown analysis mode %q", cfg.AnalysisMode)
	}
	return nil
}

func (c FileConfig) Apply(cfg *Config) {
	if c.Profile != "" {
		cfg.Profile = c.Profile
	}
	for _, id := range c.EnabledChecks {
		cfg.EnabledChecks = append(cfg.EnabledChecks, CheckID(id))
	}
	cfg.DIPAllowDependencies = append([]string(nil), c.AllowDependencies...)
	if len(c.OCPDiscriminatorFields) > 0 {
		cfg.OCPDiscriminatorFields = append([]string(nil), c.OCPDiscriminatorFields...)
	}
	cfg.OCPAllowDispatchTypes = append([]string(nil), c.OCPAllowDispatchTypes...)
	cfg.OCPAllowPackages = append([]string(nil), c.OCPAllowPackages...)
	cfg.OCPLogicPackages = append([]string(nil), c.OCPLogicPackages...)
	cfg.OCPImplementationPackages = append([]string(nil), c.OCPImplementationPackages...)
	cfg.OCPCompositionRoots = append([]string(nil), c.OCPCompositionRoots...)
	cfg.DIPInfraErrorPackages = append([]string(nil), c.DIPInfraErrorPackages...)
	cfg.DIPTransportTypes = append([]string(nil), c.DIPTransportTypes...)
	for key, value := range c.Thresholds {
		applyThresholdValue(cfg, key, value)
	}
	for _, id := range c.DisabledChecks {
		cfg.DisabledChecks = append(cfg.DisabledChecks, CheckID(id))
	}
}

// ApplyThresholds validates and applies canonical threshold keys.
func ApplyThresholds(cfg *Config, thresholds map[string]int) error {
	for key, value := range thresholds {
		if !validThreshold(key) {
			return fmt.Errorf("unknown threshold %q", key)
		}
		if err := validateThresholdValue(key, value); err != nil {
			return err
		}
		applyThresholdValue(cfg, key, value)
	}
	return ValidateConfig(*cfg)
}

func applyThresholdValue(cfg *Config, key string, value int) {
	if spec, ok := thresholdSpec(key); ok {
		spec.set(cfg, value)
	}
}

func ApplySeverity(issues []Issue, overrides map[string]Severity) {
	for n := range issues {
		if severity, ok := overrides[issues[n].ID()]; ok {
			issues[n].Severity = severity
			continue
		}
		if severity, ok := overrides[string(issues[n].Rule)]; ok {
			issues[n].Severity = severity
		}
	}
}

func validSeverity(value Severity) bool {
	return value == SeverityNote || value == SeverityWarning || value == SeverityError
}

func Excluded(path string, patterns []string) bool {
	path = filepath.ToSlash(path)
	for _, p := range patterns {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "/**") && pathMatchesDirPrefix(path, strings.TrimSuffix(p, "/**")) {
			return true
		}
		if doublestarMatch(p, path) {
			return true
		}
	}
	return false
}

// ValidateExcludePatterns rejects malformed glob segments instead of treating
// them as silent non-matches at runtime.
func ValidateExcludePatterns(patterns []string) error {
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if err := validateExcludePattern(pattern); err != nil {
			return err
		}
	}
	return nil
}

func validateExcludePattern(pattern string) error {
	pattern = strings.Trim(pattern, "/")
	if pattern == "" {
		return fmt.Errorf("exclude pattern is empty")
	}
	for _, part := range strings.Split(pattern, "/") {
		if part == "" || part == "**" {
			continue
		}
		if _, err := filepath.Match(part, "x"); err != nil {
			return fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
		}
	}
	return nil
}

func doublestarMatch(pattern, value string) bool {
	pattern = strings.Trim(pattern, "/")
	value = strings.Trim(value, "/")
	if pattern == value {
		return true
	}
	patternParts := strings.Split(pattern, "/")
	valueParts := strings.Split(value, "/")
	var match func(int, int) bool
	match = func(pi, vi int) bool {
		for pi < len(patternParts) {
			part := patternParts[pi]
			if part == "**" {
				if pi == len(patternParts)-1 {
					return true
				}
				for next := vi; next <= len(valueParts); next++ {
					if match(pi+1, next) {
						return true
					}
				}
				return false
			}
			if vi >= len(valueParts) {
				return false
			}
			matched, err := filepath.Match(part, valueParts[vi])
			if err != nil || !matched {
				return false
			}
			pi++
			vi++
		}
		return vi == len(valueParts)
	}
	if match(0, 0) {
		return true
	}
	// A non-anchored directory pattern such as generated/** has historically
	// matched the same directory at any depth. Preserve that useful behavior.
	for index := 1; index < len(valueParts); index++ {
		if match(0, index) {
			return true
		}
	}
	return false
}

func pathMatchesDirPrefix(path, prefix string) bool {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return false
	}
	if path == prefix {
		return true
	}
	if strings.HasPrefix(path, prefix+"/") {
		return true
	}
	return strings.Contains(path, "/"+prefix+"/")
}
