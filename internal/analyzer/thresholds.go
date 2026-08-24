package analyzer

import "sort"

// ThresholdSpec is the authoritative description, validation range, and
// Config binding for one public threshold key.
type ThresholdSpec struct {
	Name        string
	Description string
	Minimum     int
	Maximum     *int
	get         func(Config) int
	set         func(*Config, int)
}

var percentageMaximum = 100

var thresholdRegistry = []ThresholdSpec{
	threshold("max_methods", "Maximum methods owned by one type.", 1, nil, func(c Config) int { return c.MaxMethodsPerType }, func(c *Config, v int) { c.MaxMethodsPerType = v }),
	threshold("max_func_lines", "Maximum source lines in one function.", 1, nil, func(c Config) int { return c.MaxFuncLines }, func(c *Config, v int) { c.MaxFuncLines = v }),
	threshold("max_params", "Maximum parameters before mixed-input analysis.", 1, nil, func(c Config) int { return c.MaxFuncParams }, func(c *Config, v int) { c.MaxFuncParams = v }),
	threshold("max_switch_cases", "Maximum cases in a concrete dispatch.", 1, nil, func(c Config) int { return c.MaxTypeSwitchCases }, func(c *Config, v int) { c.MaxTypeSwitchCases = v }),
	threshold("max_interface_methods", "Maximum methods on an interface.", 1, nil, func(c Config) int { return c.MaxInterfaceMethods }, func(c *Config, v int) { c.MaxInterfaceMethods = v }),
	threshold("isp_min_methods", "Minimum interface size for ISP usage analysis.", 1, nil, func(c Config) int { return c.ISPMinMethods }, func(c *Config, v int) { c.ISPMinMethods = v }),
	threshold(thresholdISPUsage, "Minimum used-method percentage accepted by ISP.", 0, &percentageMaximum, func(c Config) int { return c.ISPUsageRatioPercent }, func(c *Config, v int) { c.ISPUsageRatioPercent = v }),
	threshold("max_fields", "Maximum fields on one type.", 1, nil, func(c Config) int { return c.MaxFieldsPerType }, func(c *Config, v int) { c.MaxFieldsPerType = v }),
	threshold("max_type_lines", "Maximum source lines in one type.", 1, nil, func(c Config) int { return c.MaxTypeLines }, func(c *Config, v int) { c.MaxTypeLines = v }),
	threshold("max_exported_methods", "Maximum exported methods on one type.", 1, nil, func(c Config) int { return c.MaxExportedMethods }, func(c *Config, v int) { c.MaxExportedMethods = v }),
	threshold("max_func_complexity", "Maximum cyclomatic complexity for a function.", 1, nil, func(c Config) int { return c.MaxFuncComplexity }, func(c *Config, v int) { c.MaxFuncComplexity = v }),
	threshold("max_type_complexity", "Maximum aggregate complexity for a type.", 1, nil, func(c Config) int { return c.MaxTypeComplexity }, func(c *Config, v int) { c.MaxTypeComplexity = v }),
	threshold("max_fanout", "Maximum external-symbol fan-out for a type.", 1, nil, func(c Config) int { return c.MaxFanOut }, func(c *Config, v int) { c.MaxFanOut = v }),
	threshold("max_atfd", "Maximum access to foreign data for a type.", 1, nil, func(c Config) int { return c.MaxATFD }, func(c *Config, v int) { c.MaxATFD = v }),
	threshold("min_large_type_signals", "Minimum large-type signals required for a finding.", 1, nil, func(c Config) int { return c.MinLargeTypeSignals }, func(c *Config, v int) { c.MinLargeTypeSignals = v }),
	threshold(thresholdMinTCC, "Minimum tight-class-cohesion percentage.", 0, &percentageMaximum, func(c Config) int { return c.MinTCCPercent }, func(c *Config, v int) { c.MinTCCPercent = v }),
	threshold("min_cohesion_methods", "Minimum methods for cohesion analysis.", 1, nil, func(c Config) int { return c.MinCohesionMethods }, func(c *Config, v int) { c.MinCohesionMethods = v }),
	threshold("min_cohesion_fields", "Minimum fields for cohesion analysis.", 1, nil, func(c Config) int { return c.MinCohesionFields }, func(c *Config, v int) { c.MinCohesionFields = v }),
	threshold("min_component_methods", "Minimum methods in a disconnected component.", 1, nil, func(c Config) int { return c.MinComponentMethods }, func(c *Config, v int) { c.MinComponentMethods = v }),
	threshold("min_import_cluster_methods", "Minimum methods in an import cluster.", 1, nil, func(c Config) int { return c.MinImportClusterMethods }, func(c *Config, v int) { c.MinImportClusterMethods = v }),
	threshold("ocp_min_dispatch_sites", "Minimum repeated dispatch sites.", 1, nil, func(c Config) int { return c.OCPMinDispatchSites }, func(c *Config, v int) { c.OCPMinDispatchSites = v }),
	threshold("ocp_min_shared_variants", "Minimum variants shared by dispatch sites.", 1, nil, func(c Config) int { return c.OCPMinSharedVariants }, func(c *Config, v int) { c.OCPMinSharedVariants = v }),
	threshold(thresholdOCPOverlap, "Minimum dispatch-variant overlap percentage.", 0, &percentageMaximum, func(c Config) int { return c.OCPDispatchOverlapPercent }, func(c *Config, v int) { c.OCPDispatchOverlapPercent = v }),
	threshold("ocp_min_concrete_parameter_methods", "Minimum methods on a concrete parameter type.", 1, nil, func(c Config) int { return c.OCPMinConcreteParameterMethods }, func(c *Config, v int) { c.OCPMinConcreteParameterMethods = v }),
	threshold("ocp_min_implementation_imports", "Minimum implementation imports for coupling analysis.", 1, nil, func(c Config) int { return c.OCPMinImplementationImports }, func(c *Config, v int) { c.OCPMinImplementationImports = v }),
	threshold("ocp_min_parallel_functions", "Minimum functions in a parallel implementation group.", 1, nil, func(c Config) int { return c.OCPMinParallelFunctions }, func(c *Config, v int) { c.OCPMinParallelFunctions = v }),
	threshold("ocp_min_parallel_nodes", "Minimum syntax nodes in a parallel implementation.", 1, nil, func(c Config) int { return c.OCPMinParallelNodes }, func(c *Config, v int) { c.OCPMinParallelNodes = v }),
	threshold(thresholdOCPSimilarity, "Minimum parallel-implementation similarity percentage.", 0, &percentageMaximum, func(c Config) int { return c.OCPParallelSimilarityPercent }, func(c *Config, v int) { c.OCPParallelSimilarityPercent = v }),
	threshold("dip_composition_root_fields", "Minimum concrete fields indicating a composition root.", 1, nil, func(c Config) int { return c.DIPCompositionRootFields }, func(c *Config, v int) { c.DIPCompositionRootFields = v }),
}

func threshold(name, description string, minimum int, maximum *int, get func(Config) int, set func(*Config, int)) ThresholdSpec {
	return ThresholdSpec{Name: name, Description: description, Minimum: minimum, Maximum: maximum, get: get, set: set}
}

// ThresholdSpecs returns the registry in canonical configuration order.
func ThresholdSpecs() []ThresholdSpec {
	specs := append([]ThresholdSpec(nil), thresholdRegistry...)
	for index := range specs {
		if specs[index].Maximum != nil {
			maximum := *specs[index].Maximum
			specs[index].Maximum = &maximum
		}
	}
	return specs
}

// EffectiveThresholds returns canonical names and current values.
func EffectiveThresholds(cfg Config) map[string]int {
	values := make(map[string]int, len(thresholdRegistry))
	for _, spec := range thresholdRegistry {
		values[spec.Name] = spec.get(cfg)
	}
	return values
}

func thresholdSpec(name string) (ThresholdSpec, bool) {
	for _, spec := range thresholdRegistry {
		if spec.Name == name {
			return spec, true
		}
	}
	return ThresholdSpec{}, false
}

func thresholdNames() []string {
	names := make([]string, len(thresholdRegistry))
	for index, spec := range thresholdRegistry {
		names[index] = spec.Name
	}
	sort.Strings(names)
	return names
}
