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
	switch key {
	case "max_methods", "max_func_lines", "max_params", "max_switch_cases", "max_interface_methods",
		"isp_min_methods", thresholdISPUsage, "max_fields", "max_type_lines", "max_exported_methods",
		"max_func_complexity", "max_type_complexity", "max_fanout", "max_atfd",
		"min_large_type_signals", thresholdMinTCC, "min_cohesion_methods", "min_cohesion_fields",
		"min_component_methods", "min_import_cluster_methods", "ocp_min_dispatch_sites",
		"ocp_min_shared_variants", thresholdOCPOverlap, "ocp_min_concrete_parameter_methods",
		"ocp_min_implementation_imports", "ocp_min_parallel_functions", "ocp_min_parallel_nodes",
		thresholdOCPSimilarity, "dip_composition_root_fields":
		return true
	}
	return false
}

func validateThresholdValue(key string, value int) error {
	if value < 0 {
		return fmt.Errorf("threshold %q must be non-negative", key)
	}
	switch key {
	case thresholdISPUsage, thresholdMinTCC, thresholdOCPOverlap, thresholdOCPSimilarity:
		if value > 100 {
			return fmt.Errorf("threshold %q must be between 0 and 100", key)
		}
		return nil
	default:
		if value < 1 {
			return fmt.Errorf("threshold %q must be at least 1", key)
		}
		return nil
	}
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
	values := map[string]int{
		"max_methods": cfg.MaxMethodsPerType, "max_func_lines": cfg.MaxFuncLines,
		"max_params": cfg.MaxFuncParams, "max_fields": cfg.MaxFieldsPerType,
		"max_type_lines": cfg.MaxTypeLines, "max_exported_methods": cfg.MaxExportedMethods,
		"max_func_complexity": cfg.MaxFuncComplexity, "max_type_complexity": cfg.MaxTypeComplexity,
		"max_fanout": cfg.MaxFanOut, "max_atfd": cfg.MaxATFD,
		"min_large_type_signals": cfg.MinLargeTypeSignals, "min_cohesion_methods": cfg.MinCohesionMethods,
		"min_cohesion_fields": cfg.MinCohesionFields, "min_component_methods": cfg.MinComponentMethods,
		"min_import_cluster_methods": cfg.MinImportClusterMethods, "max_switch_cases": cfg.MaxTypeSwitchCases,
		"ocp_min_dispatch_sites": cfg.OCPMinDispatchSites, "ocp_min_shared_variants": cfg.OCPMinSharedVariants,
		"ocp_min_concrete_parameter_methods": cfg.OCPMinConcreteParameterMethods,
		"ocp_min_implementation_imports":     cfg.OCPMinImplementationImports,
		"ocp_min_parallel_functions":         cfg.OCPMinParallelFunctions, "ocp_min_parallel_nodes": cfg.OCPMinParallelNodes,
		"max_interface_methods": cfg.MaxInterfaceMethods, "isp_min_methods": cfg.ISPMinMethods,
		"dip_composition_root_fields": cfg.DIPCompositionRootFields,
	}
	for key, value := range values {
		if value < 1 {
			return fmt.Errorf("threshold %q must be at least 1", key)
		}
	}
	percentages := map[string]int{
		thresholdISPUsage:      cfg.ISPUsageRatioPercent,
		thresholdMinTCC:        cfg.MinTCCPercent,
		thresholdOCPOverlap:    cfg.OCPDispatchOverlapPercent,
		thresholdOCPSimilarity: cfg.OCPParallelSimilarityPercent,
	}
	for key, value := range percentages {
		if value < 0 || value > 100 {
			return fmt.Errorf("threshold %q must be between 0 and 100", key)
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
	switch key {
	case "max_methods":
		cfg.MaxMethodsPerType = value
	case "max_func_lines":
		cfg.MaxFuncLines = value
	case "max_params":
		cfg.MaxFuncParams = value
	case "max_switch_cases":
		cfg.MaxTypeSwitchCases = value
	case "max_interface_methods":
		cfg.MaxInterfaceMethods = value
	case "isp_min_methods":
		cfg.ISPMinMethods = value
	case thresholdISPUsage:
		cfg.ISPUsageRatioPercent = value
	case "max_fields":
		cfg.MaxFieldsPerType = value
	case "max_type_lines":
		cfg.MaxTypeLines = value
	case "max_exported_methods":
		cfg.MaxExportedMethods = value
	case "max_func_complexity":
		cfg.MaxFuncComplexity = value
	case "max_type_complexity":
		cfg.MaxTypeComplexity = value
	case "max_fanout":
		cfg.MaxFanOut = value
	case "max_atfd":
		cfg.MaxATFD = value
	case "min_large_type_signals":
		cfg.MinLargeTypeSignals = value
	case thresholdMinTCC:
		cfg.MinTCCPercent = value
	case "min_cohesion_methods":
		cfg.MinCohesionMethods = value
	case "min_cohesion_fields":
		cfg.MinCohesionFields = value
	case "min_component_methods":
		cfg.MinComponentMethods = value
	case "min_import_cluster_methods":
		cfg.MinImportClusterMethods = value
	case "dip_composition_root_fields":
		cfg.DIPCompositionRootFields = value
	case "ocp_min_dispatch_sites":
		cfg.OCPMinDispatchSites = value
	case "ocp_min_shared_variants":
		cfg.OCPMinSharedVariants = value
	case thresholdOCPOverlap:
		cfg.OCPDispatchOverlapPercent = value
	case "ocp_min_concrete_parameter_methods":
		cfg.OCPMinConcreteParameterMethods = value
	case "ocp_min_implementation_imports":
		cfg.OCPMinImplementationImports = value
	case "ocp_min_parallel_functions":
		cfg.OCPMinParallelFunctions = value
	case "ocp_min_parallel_nodes":
		cfg.OCPMinParallelNodes = value
	case thresholdOCPSimilarity:
		cfg.OCPParallelSimilarityPercent = value
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
