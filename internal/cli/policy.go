package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/ExtroNovosib/solidify/internal/analyzer"
	configpkg "github.com/ExtroNovosib/solidify/internal/config"
)

type checkPolicy struct {
	options    checkOptions
	config     analyzer.Config
	fileConfig analyzer.FileConfig
	enabled    map[analyzer.Rule]bool
	plan       analyzer.ExecutionPlan
	configFile string
}

func resolveCheckPolicy(options checkOptions, build BuildInfo) (checkPolicy, error) {
	cfg := analyzer.DefaultConfig()
	cfg.Profile = analyzer.ProfileStable
	configFile := options.configPath
	discovered := false
	var err error
	if configFile == "" {
		configFile, err = configpkg.FindForTargets(options.paths)
		if err != nil {
			return checkPolicy{}, err
		}
		discovered = configFile != ""
	}
	var fileConfig analyzer.FileConfig
	if configFile != "" {
		fileConfig, err = configpkg.Load(configFile)
		if err != nil {
			return checkPolicy{}, err
		}
		fileConfig.Apply(&cfg)
		if discovered {
			fmt.Fprintln(os.Stderr, "solidlint: using config", configFile, "(discovered from scan target)")
		}
	}
	applyCheckOverrides(&cfg, &options, fileConfig)
	cfg.CacheDir = options.cacheDir
	cfg.CacheEnabled = options.cache
	cfg.CacheDiagnostics = options.cacheDebug
	cfg.AnalysisMode = options.analysis
	cfg.IncludeTests = options.includeTests
	cfg.ToolVersion = build.Version
	cfg.ExcludedFiles = append([]string(nil), fileConfig.Excludes...)
	if err := analyzer.ValidateConfig(cfg); err != nil {
		return checkPolicy{}, err
	}
	ruleSpec := options.rules
	if len(fileConfig.EnabledRules) > 0 && !options.set["rules"] {
		ruleSpec = strings.Join(fileConfig.EnabledRules, ",")
	}
	enabled, err := parseRules(ruleSpec)
	if err != nil {
		return checkPolicy{}, err
	}
	plan, err := analyzer.NewExecutionPlan(cfg, enabled, analyzer.SurfaceCLI)
	if err != nil {
		return checkPolicy{}, err
	}
	return checkPolicy{options: options, config: cfg, fileConfig: fileConfig, enabled: enabled, plan: plan, configFile: configFile}, nil
}

func applyCheckOverrides(cfg *analyzer.Config, options *checkOptions, fileConfig analyzer.FileConfig) {
	if options.set["max-methods"] {
		cfg.MaxMethodsPerType = options.maxMethods
	}
	if options.set["max-func-lines"] {
		cfg.MaxFuncLines = options.maxFuncLines
	}
	if options.set["max-params"] {
		cfg.MaxFuncParams = options.maxParams
	}
	if options.set["max-switch-cases"] {
		cfg.MaxTypeSwitchCases = options.maxSwitchCases
	}
	if options.set["max-interface-methods"] {
		cfg.MaxInterfaceMethods = options.maxInterfaceMethods
	}
	if options.set["isp-min-methods"] {
		cfg.ISPMinMethods = options.ispMinMethods
	}
	if options.set["isp-usage-ratio-percent"] {
		cfg.ISPUsageRatioPercent = options.ispUsageRatioPercent
	}
	if fileConfig.FailLevel != "" && !options.set["fail-level"] {
		options.failLevel = string(fileConfig.FailLevel)
	}
	if options.set["profile"] || fileConfig.Profile == "" {
		cfg.Profile = analyzer.Profile(options.profile)
	}
	if options.set["enable-checks"] {
		cfg.EnabledChecks = parseCheckIDs(options.enabledChecks)
	}
}

type analysisResult struct {
	issues   []analyzer.Issue
	stats    analyzer.ExecutionStats
	warnings []string
}

func executeAnalysis(policy checkPolicy) (analysisResult, error) {
	packages, warnings, err := analyzer.LoadWorkspace(policy.options.paths, policy.options.includeTests, policy.options.analysis)
	if err != nil {
		return analysisResult{}, err
	}
	if err := analyzer.ValidateSuppressions(packages); err != nil {
		return analysisResult{}, err
	}
	issues, stats := analyzer.RunPlan(packages, policy.config, policy.plan)
	if err := analyzer.FinalizeIssues(issues, "workspace"); err != nil {
		return analysisResult{}, fmt.Errorf("internal analysis error: %w", err)
	}
	analyzer.ApplySeverity(issues, policy.fileConfig.Severities)
	issues = filterIssues(issues, policy.fileConfig.Excludes)
	return analysisResult{issues: issues, stats: stats, warnings: warnings}, nil
}
